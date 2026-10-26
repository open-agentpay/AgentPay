import sys
import unittest
from pathlib import Path

# Add repo root to sys.path to import scripts.validate_vectors
repo_root = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(repo_root))

from scripts.validate_vectors import (
    validate_eip712_vectors,
    validate_policy_budget_vectors,
    validate_x402_vectors,
)


class TestConformanceVectors(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # Support both new specs/test-vectors and legacy specs/conformance/test-vectors
        primary = repo_root / "specs" / "test-vectors"
        legacy = repo_root / "specs" / "conformance" / "test-vectors"
        cls.vectors_dir = primary if primary.exists() else legacy
        assert cls.vectors_dir.exists(), f"Vectors directory not found at {cls.vectors_dir}"

    def test_eip712_authorization_vectors(self):
        """Validates EIP-712 structured data and cryptographic signatures."""
        errors = validate_eip712_vectors(self.vectors_dir)
        self.assertEqual(errors, 0, f"EIP-712 conformance vectors had {errors} error(s)")

    def test_policy_budget_vectors(self):
        """Validates policy budget test vectors integrity and mathematical invariants."""
        errors = validate_policy_budget_vectors(self.vectors_dir)
        self.assertEqual(errors, 0, f"Policy budget conformance vectors had {errors} error(s)")

    def test_x402_v2_challenge_vectors(self):
        """Validates x402 challenge schema and CAIP-2 regex constraints."""
        errors = validate_x402_vectors(self.vectors_dir)
        self.assertEqual(errors, 0, f"x402 challenge conformance vectors had {errors} error(s)")

    def test_x402_native_models_conformance(self):
        """Validates x402 native Pydantic V2 models against conformance JSON test vectors."""
        import json

        from agentpay.core.codecs import (
            decode_authorization,
            decode_challenge,
            encode_authorization,
            encode_challenge,
        )
        from agentpay.core.errors import X402ValidationError
        from agentpay.core.models import PaymentAuthorization, PaymentChallenge

        filepath = self.vectors_dir / "x402-v2-challenge.json"
        with open(filepath, "r", encoding="utf-8") as f:
            data = json.load(f)

        for item in data["validChallenges"]:
            payload = item["payload"]
            expected_header = item["expectedHeader"]
            if expected_header == "PAYMENT-REQUIRED":
                challenge = PaymentChallenge.model_validate(payload)
                self.assertEqual(challenge.version, "2.0")
                # Test roundtrip encoding and decoding
                encoded = encode_challenge(challenge)
                decoded = decode_challenge(encoded)
                self.assertEqual(challenge, decoded)
            elif expected_header == "PAYMENT-SIGNATURE":
                auth = PaymentAuthorization.model_validate(payload)
                self.assertEqual(auth.version, "2.0")
                # Test roundtrip encoding and decoding
                encoded = encode_authorization(auth)
                decoded = decode_authorization(encoded)
                self.assertEqual(auth, decoded)

        for item in data["invalidChallenges"]:
            payload = item["payload"]
            expected_error = item["expectedError"]
            with self.assertRaises(X402ValidationError) as ctx:
                PaymentChallenge.model_validate(payload)
            self.assertEqual(ctx.exception.code.value, expected_error)


if __name__ == "__main__":
    unittest.main()
