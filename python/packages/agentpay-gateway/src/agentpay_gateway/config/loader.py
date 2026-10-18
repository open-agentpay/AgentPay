"""
YAML Configuration Loader with Environment Variable Interpolation.

Supports:
- Safe YAML loading via yaml.safe_load (zero code execution).
- Dynamic environment variable substitution:
  - ${VAR_NAME} -> os.environ[VAR_NAME]
  - ${VAR_NAME:-default_value} -> os.environ.get(VAR_NAME, default_value)
  - ${VAR_NAME:default_value} -> os.environ.get(VAR_NAME, default_value)
- Loading from file path, Path object, or raw YAML string.
"""

from __future__ import annotations

import os
import re
from pathlib import Path
from typing import Any

from agentpay_gateway.config.errors import (
    EnvInterpolationError,
    GatewayConfigError,
    GatewayConfigFileNotFoundError,
    GatewayConfigValidationError,
)
from agentpay_gateway.config.models import GatewayConfig

try:
    import yaml
except ImportError:
    try:
        from agentpay_gateway._vendor import yaml
    except ImportError:
        yaml = None

ENV_PATTERN = re.compile(r"\$\{([A-Za-z0-9_]+)(?::-(.*?)|:(.*?))?\}")


def interpolate_env_vars(text: str) -> str:
    """
    Interpolate environment variables into configuration text.

    Syntax supported:
      - ${FOO} -> os.environ["FOO"] (raises EnvInterpolationError if missing)
      - ${FOO:-default} -> os.environ.get("FOO", "default")
      - ${FOO:default} -> os.environ.get("FOO", "default")

    Args:
        text: Raw configuration text.

    Returns:
        Interpolated text.

    Raises:
        EnvInterpolationError: If a variable without a default value is not set.
    """

    def replace_match(match: re.Match[str]) -> str:
        var_name = match.group(1)
        default_dash = match.group(2)
        default_colon = match.group(3)
        default_val = default_dash if default_dash is not None else default_colon

        if var_name in os.environ:
            return os.environ[var_name]

        if default_val is not None:
            return default_val

        raise EnvInterpolationError(
            f"Missing required environment variable '{var_name}' without default value in configuration"
        )

    return ENV_PATTERN.sub(replace_match, text)


def load_gateway_config_from_dict(data: dict[str, Any]) -> GatewayConfig:
    """
    Instantiate and validate a GatewayConfig from a Python dictionary.

    Args:
        data: Parsed configuration dictionary.

    Returns:
        Validated GatewayConfig instance.
    """
    if not isinstance(data, dict):
        raise GatewayConfigValidationError(f"Configuration root must be a mapping/dict, got {type(data).__name__}")
    try:
        return GatewayConfig(**data)
    except Exception as exc:
        if isinstance(exc, GatewayConfigError):
            raise
        raise GatewayConfigValidationError(f"Invalid gateway configuration: {exc}") from exc


def load_gateway_config_from_yaml(path_or_content: str | Path) -> GatewayConfig:
    """
    Load, interpolate, and validate GatewayConfig from a YAML file or raw YAML string.

    Args:
        path_or_content: File path (str or Path) or inline YAML content.

    Returns:
        Validated GatewayConfig instance.

    Raises:
        GatewayConfigFileNotFoundError: If path does not exist.
        GatewayConfigValidationError: If YAML is malformed or invalid according to schema.
        EnvInterpolationError: If required env variable is unset.
    """
    if yaml is None:
        raise GatewayConfigError("PyYAML is not installed or available in gateway dependencies.")

    content = ""

    if isinstance(path_or_content, Path):
        if not path_or_content.is_file():
            raise GatewayConfigFileNotFoundError(f"Gateway configuration file not found: '{path_or_content}'")
        try:
            with open(path_or_content, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError as err:
            raise GatewayConfigError(f"Failed to read configuration file '{path_or_content}': {err}") from err
    elif isinstance(path_or_content, str):
        # Determine whether path_or_content is a file path or direct YAML content
        # If it doesn't contain newlines and either exists as a file or has a yaml/yml suffix or contains path separators:
        if "\n" not in path_or_content:
            candidate_path = Path(path_or_content)
            if candidate_path.is_file():
                try:
                    with open(candidate_path, "r", encoding="utf-8") as f:
                        content = f.read()
                except OSError as err:
                    raise GatewayConfigError(f"Failed to read configuration file '{candidate_path}': {err}") from err
            elif path_or_content.endswith((".yaml", ".yml")) or "/" in path_or_content or "\\" in path_or_content:
                raise GatewayConfigFileNotFoundError(f"Gateway configuration file not found: '{path_or_content}'")
            else:
                content = path_or_content
        else:
            content = path_or_content
    else:
        raise GatewayConfigValidationError(f"Expected str or Path, got {type(path_or_content).__name__}")

    # Step 1: Interpolate environment variables
    interpolated = interpolate_env_vars(content)

    # Step 2: Safe YAML parsing
    try:
        parsed_data = yaml.safe_load(interpolated)
    except yaml.YAMLError as exc:
        raise GatewayConfigValidationError(f"YAML parsing error: {exc}") from exc

    if not isinstance(parsed_data, dict):
        raise GatewayConfigValidationError(
            f"Gateway YAML must deserialize to a top-level mapping/dictionary, got {type(parsed_data).__name__}"
        )

    # Step 3: Validate with Pydantic model
    return load_gateway_config_from_dict(parsed_data)


# Friendly alias
load_gateway_config = load_gateway_config_from_yaml
