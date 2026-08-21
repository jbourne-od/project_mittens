"""Exceptions for Project Mittens Python SDK."""

from typing import Optional, Dict, Any


class MittensError(Exception):
    """Base exception for all Project Mittens SDK errors."""
    pass


class MittensAPIError(MittensError):
    """Exception raised when the Go Optimization Engine returns a 4xx or 5xx HTTP response."""

    def __init__(
        self,
        status_code: int,
        error_code: str,
        message: str,
        details: Optional[Dict[str, Any]] = None,
    ) -> None:
        self.status_code = status_code
        self.error_code = error_code
        self.message = message
        self.details = details or {}
        super().__init__(f"[{status_code}] {error_code}: {message}")


class ReplayMismatchError(MittensError):
    """Exception raised when deterministic state replay fails cryptographic hash verification."""

    def __init__(self, decision_id: str, drift: float, message: str) -> None:
        self.decision_id = decision_id
        self.drift = drift
        self.message = message
        super().__init__(
            f"Cryptographic replay mismatch for decision {decision_id} (drift={drift:.6f}): {message}"
        )


class ChainIntegrityError(MittensError):
    """Exception raised when the SHA-256 Merkle chain is broken or corrupted."""

    def __init__(self, run_id: str, broken_record_id: str, message: str) -> None:
        self.run_id = run_id
        self.broken_record_id = broken_record_id
        self.message = message
        super().__init__(
            f"Merkle chain corruption in run {run_id} at record {broken_record_id}: {message}"
        )
