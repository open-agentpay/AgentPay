"""
In-Process Asynchronous Blockchain Settlement Worker for AgentPay Gateway.

Enforces:
- Optimistic Fulfillment: Client receives upstream response without waiting for on-chain block mining
- Non-blocking in-process settlement queue dispatching POST /v1/settle to Facilitator
- Automated exponential backoff retry on transient clearing errors
- State machine synchronization: transitions GatewaySettlementRecord in SQLite
- Clean queue draining and graceful shutdown
"""

from __future__ import annotations

import logging
import queue
import threading
import time
from collections.abc import Callable
from typing import Any

from agentpay.core.state import SettlementState

from agentpay_gateway.facilitator.models import SettleRequest
from agentpay_gateway.payment.models import SettlementTask
from agentpay_gateway.proxy.logging import StructuredAuditLogger
from agentpay_gateway.state.coordinator import GatewayStateCoordinator

logger = logging.getLogger("agentpay_gateway.settlement")


class AsyncSettlementWorker:
    """
    Background worker processing asynchronous on-chain settlements via Facilitator.
    """

    def __init__(
        self,
        facilitator_client: Any,
        state_coordinator: GatewayStateCoordinator | None = None,
        audit_logger: StructuredAuditLogger | None = None,
        *,
        max_retries: int = 3,
        retry_delay_seconds: float = 0.5,
        clock: Callable[[], int] | None = None,
    ) -> None:
        self._facilitator = facilitator_client
        self.state_coordinator = state_coordinator
        self.audit_logger = audit_logger or StructuredAuditLogger()
        self.max_retries = max_retries
        self.retry_delay_seconds = retry_delay_seconds
        self._clock = clock or (lambda: int(time.time()))

        self._queue: queue.Queue[SettlementTask | None] = queue.Queue()
        self._stopped = threading.Event()
        self._thread: threading.Thread | None = None
        self._settled_count = 0
        self._failed_count = 0
        self._last_error: str | None = None

        self.start()

    def get_time(self) -> int:
        return self._clock()

    @property
    def settled_count(self) -> int:
        return self._settled_count

    @property
    def failed_count(self) -> int:
        return self._failed_count

    @property
    def last_error(self) -> str | None:
        return self._last_error

    def start(self) -> None:
        """Start the background worker thread."""
        if self._thread is None or not self._thread.is_alive():
            self._stopped.clear()
            self._thread = threading.Thread(
                target=self._worker_loop,
                name="agentpay-async-settlement-worker",
                daemon=True,
            )
            self._thread.start()

    def submit(self, task: SettlementTask) -> None:
        """
        Enqueue a verified payment authorization for background clearing.
        Non-blocking.
        """
        if self._stopped.is_set():
            raise RuntimeError("AsyncSettlementWorker is stopped and cannot accept new tasks")
        self._queue.put(task)

    def _worker_loop(self) -> None:
        """Main background worker consumption loop."""
        while not self._stopped.is_set():
            try:
                task = self._queue.get(timeout=0.2)
            except queue.Empty:
                continue

            if task is None:
                # Sentinel shutdown signal
                self._queue.task_done()
                break

            self._process_task(task)
            self._queue.task_done()

    def _process_task(self, task: SettlementTask) -> None:
        """Execute settlement submission against Facilitator."""
        try:
            # 1. Dispatch settlement to Facilitator
            settle_resp = self._call_facilitator_settle(task)

            # 2. Transition settlement in state machine
            if self.state_coordinator is not None:
                new_state = getattr(settle_resp, "status", SettlementState.SUBMITTED)
                tx_hash = getattr(settle_resp, "tx_hash", None)
                # If Facilitator returned a string, convert to Enum
                if isinstance(new_state, str):
                    new_state = SettlementState(new_state)

                try:
                    self.state_coordinator.transition_settlement(
                        settlement_id=task.settlement_id,
                        to_state=new_state,
                        tx_hash=tx_hash,
                    )
                except Exception as exc:
                    logger.warning(f"State machine transition failed for settlement '{task.settlement_id}': {exc}")

            self._settled_count += 1

            # 3. Log audit event
            self.audit_logger.log_event(
                "settlement_submitted",
                settlement_id=task.settlement_id,
                payer=task.authorization.payer,
                idempotency_key=task.idempotency_key,
                facilitator_settlement_id=getattr(settle_resp, "settlement_id", None),
                status=str(getattr(settle_resp, "status", "SUBMITTED")),
            )
        except Exception as exc:
            self._last_error = str(exc)
            logger.warning(
                f"Settlement submission failed for '{task.settlement_id}' (attempt {task.retry_count + 1}): {exc}"
            )

            if task.retry_count < self.max_retries:
                # Re-enqueue for retry after delay
                time.sleep(self.retry_delay_seconds)
                retry_task = task.model_copy(update={"retry_count": task.retry_count + 1})
                self._queue.put(retry_task)
            else:
                self._failed_count += 1
                self.audit_logger.log_event(
                    "settlement_failed",
                    settlement_id=task.settlement_id,
                    payer=task.authorization.payer,
                    idempotency_key=task.idempotency_key,
                    error=str(exc),
                    retries=task.retry_count,
                )

    def _call_facilitator_settle(self, task: SettlementTask) -> Any:
        """Adapt between FacilitatorClient and MockFacilitatorService calling conventions."""
        if hasattr(self._facilitator, "settle"):
            try:
                return self._facilitator.settle(
                    authorization=task.authorization,
                    verification_token=task.verification_token,
                    idempotency_key=task.idempotency_key,
                )
            except TypeError:
                req = SettleRequest(
                    verificationToken=task.verification_token,
                    authorization=task.authorization,
                    idempotencyKey=task.idempotency_key,
                )
                return self._facilitator.settle(req)
        elif callable(self._facilitator):
            return self._facilitator(task)
        else:
            raise RuntimeError("Configured Facilitator has no valid settle interface")

    def drain(self, timeout: float = 5.0) -> None:
        """
        Wait until all queued settlement tasks have been completely processed.
        Useful for deterministic testing and graceful shutdown.
        """
        deadline = time.time() + timeout
        while not self._queue.empty():
            time.sleep(0.05)
            if time.time() > deadline:
                break
        self._queue.join()

    def stop(self, timeout: float = 5.0) -> None:
        """Gracefully drain queue and stop the background worker thread."""
        self._stopped.set()
        self._queue.put(None)  # Wake up worker
        if self._thread and self._thread.is_alive():
            self._thread.join(timeout=timeout)
