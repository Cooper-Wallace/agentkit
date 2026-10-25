#!/usr/bin/env python3
"""
cv_splitter.py - Walk-Forward & Sequential Cross-Validation Splitter.

Provides leakage-free cross-validation splitters for time-series and sequential data,
incorporating expanding or rolling windows, purge intervals (for label overlap),
and embargo buffers (for serial autocorrelation dampening).

Compatible with scikit-learn's BaseCrossValidator interface while operating
natively with or without NumPy/Pandas installed.
"""

from __future__ import annotations

import argparse
import csv
import json
import math
import sys
from typing import Any, Generator, Iterator, List, Optional, Sequence, Tuple, Union


class WalkForwardSplitter:
    """
    Walk-forward cross-validator for time-series datasets.

    Generates train/test split indices respecting chronological ordering,
    preventing lookahead bias. Supports expanding or rolling training windows,
    purge gaps (to handle overlapping label horizons), and embargo intervals.

    Parameters
    ----------
    n_splits : int, default=5
        Number of split folds.
    train_size : int, float, or None, default=None
        Initial training window size. If float between 0 and 1, interpreted
        as fraction of total samples. If None, automatically derived from n_splits.
    test_size : int, float, or None, default=None
        Validation window size per fold. If float between 0 and 1, interpreted
        as fraction of total samples. If None, automatically derived.
    mode : {'expanding', 'rolling'}, default='expanding'
        In 'expanding' mode, the training window grows with each fold.
        In 'rolling' mode, the training window maintains a constant width.
    purge_window : int, default=0
        Number of observations discarded immediately preceding the test set
        to prevent information leakage from forward-looking labels.
    embargo_window : int, default=0
        Number of observations discarded immediately following the test set
        to mitigate serial autocorrelation contamination.
    """

    def __init__(
        self,
        n_splits: int = 5,
        train_size: Optional[Union[int, float]] = None,
        test_size: Optional[Union[int, float]] = None,
        mode: str = "expanding",
        purge_window: int = 0,
        embargo_window: int = 0,
    ) -> None:
        if n_splits < 1:
            raise ValueError(f"n_splits must be at least 1, got {n_splits}")
        if mode not in ("expanding", "rolling"):
            raise ValueError(f"mode must be 'expanding' or 'rolling', got '{mode}'")
        if purge_window < 0:
            raise ValueError(f"purge_window must be non-negative, got {purge_window}")
        if embargo_window < 0:
            raise ValueError(f"embargo_window must be non-negative, got {embargo_window}")

        self.n_splits = n_splits
        self.train_size = train_size
        self.test_size = test_size
        self.mode = mode
        self.purge_window = purge_window
        self.embargo_window = embargo_window

    def get_n_splits(self, X: Any = None, y: Any = None, groups: Any = None) -> int:
        """Return the number of splitting iterations in the cross-validator."""
        return self.n_splits

    def _resolve_sizes(self, n_samples: int) -> Tuple[int, int]:
        """Resolve integer sample counts for train_size and test_size."""
        if n_samples < 2:
            raise ValueError(f"At least 2 samples required to split, got {n_samples}")

        # Resolve test_size
        if self.test_size is None:
            # Default: allocate remaining samples after reasonable initial train size
            denom = self.n_splits + 1
            calculated_test = max(1, n_samples // denom)
            test_n = calculated_test
        elif isinstance(self.test_size, float) and 0.0 < self.test_size < 1.0:
            test_n = max(1, int(math.floor(n_samples * self.test_size)))
        elif isinstance(self.test_size, int) and self.test_size >= 1:
            test_n = self.test_size
        else:
            raise ValueError(f"Invalid test_size: {self.test_size}")

        # Resolve train_size
        if self.train_size is None:
            # Default initial train size: total minus (n_splits * test_size)
            reserved_for_tests = self.n_splits * test_n
            remaining = n_samples - reserved_for_tests
            train_n = max(1, remaining) if remaining > 0 else max(1, test_n)
        elif isinstance(self.train_size, float) and 0.0 < self.train_size < 1.0:
            train_n = max(1, int(math.floor(n_samples * self.train_size)))
        elif isinstance(self.train_size, int) and self.train_size >= 1:
            train_n = self.train_size
        else:
            raise ValueError(f"Invalid train_size: {self.train_size}")

        return train_n, test_n

    def compute_fold_ranges(self, n_samples: int) -> List[dict[str, Any]]:
        """
        Compute fold boundary metadata without generating large index lists.

        Returns list of fold dictionaries with train_start, train_end,
        purge_start, purge_end, test_start, test_end, embargo_start, embargo_end.
        """
        train_n, test_n = self._resolve_sizes(n_samples)
        folds = []

        # In sequential cross-validation, folds progress forward in time.
        # We determine the test start positions:
        step = max(1, (n_samples - train_n - test_n) // max(1, self.n_splits - 1)) if self.n_splits > 1 else 0

        for i in range(self.n_splits):
            test_start = train_n + i * step
            test_end = min(n_samples, test_start + test_n)

            if test_start >= n_samples or test_end <= test_start:
                break

            # Calculate train boundaries
            if self.mode == "expanding":
                train_start = 0
            else:  # rolling
                train_start = max(0, test_start - train_n)

            raw_train_end = test_start
            train_end = max(train_start, raw_train_end - self.purge_window)

            purge_start = train_end
            purge_end = test_start

            embargo_start = test_end
            embargo_end = min(n_samples, test_end + self.embargo_window)

            train_count = max(0, train_end - train_start)
            test_count = max(0, test_end - test_start)

            folds.append({
                "fold": i + 1,
                "train_start": train_start,
                "train_end": train_end,
                "train_count": train_count,
                "purge_start": purge_start,
                "purge_end": purge_end,
                "purge_count": purge_end - purge_start,
                "test_start": test_start,
                "test_end": test_end,
                "test_count": test_count,
                "embargo_start": embargo_start,
                "embargo_end": embargo_end,
                "embargo_count": embargo_end - embargo_start,
            })

        return folds

    def split(
        self, X: Any, y: Any = None, groups: Any = None
    ) -> Generator[Tuple[List[int], List[int]], None, None]:
        """
        Generate indices to split data into training and test set.

        Parameters
        ----------
        X : array-like or sequence
            Data to split. Must support len(X).
        y : array-like, optional
            Target variable.
        groups : array-like, optional
            Group labels.

        Yields
        ------
        train_indices : list of int
            The training set indices for that fold.
        test_indices : list of int
            The testing set indices for that fold.
        """
        n_samples = len(X)
        fold_ranges = self.compute_fold_ranges(n_samples)

        for fold in fold_ranges:
            train_idx = list(range(fold["train_start"], fold["train_end"]))
            test_idx = list(range(fold["test_start"], fold["test_end"]))
            yield train_idx, test_idx


class TimeSeriesSplitter(WalkForwardSplitter):
    """
    Alias for WalkForwardSplitter with full compatibility with scikit-learn
    time-series cross-validation interfaces.
    """
    pass


def load_csv_data(filepath: str, time_col: Optional[str] = None) -> List[dict[str, Any]]:
    """Load CSV records and optionally sort by time column."""
    with open(filepath, mode="r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        records = list(reader)

    if time_col:
        if records and time_col not in records[0]:
            raise KeyError(f"Specified time column '{time_col}' not found in CSV headers.")
        records.sort(key=lambda r: r[time_col])

    return records


def format_summary_table(folds: List[dict[str, Any]], n_samples: int) -> str:
    """Format human-readable summary table of cross-validation folds."""
    lines = [
        f"Cross-Validation Summary (Total Samples: {n_samples}, Folds: {len(folds)})",
        "=" * 84,
        f"{'Fold':<6}{'Train Range':<18}{'Train N':<10}{'Purge N':<10}{'Test Range':<18}{'Test N':<10}{'Embargo N':<10}",
        "-" * 84,
    ]
    for f in folds:
        train_range = f"[{f['train_start']}:{f['train_end']}]"
        test_range = f"[{f['test_start']}:{f['test_end']}]"
        lines.append(
            f"{f['fold']:<6}{train_range:<18}{f['train_count']:<10}{f['purge_count']:<10}"
            f"{test_range:<18}{f['test_count']:<10}{f['embargo_count']:<10}"
        )
    lines.append("=" * 84)
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Walk-Forward Sequential Cross-Validation Splitter",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument("--input", "-i", type=str, help="Input CSV dataset file path.")
    parser.add_argument("--n-samples", "-n", type=int, help="Total sample count (synthetic if no file).")
    parser.add_argument("--time-col", "-t", type=str, help="Column name to sort data chronologically.")
    parser.add_argument("--n-splits", "-k", type=int, default=5, help="Number of cross-validation folds.")
    parser.add_argument("--train-size", type=float, help="Initial training size (count or fraction 0..1).")
    parser.add_argument("--test-size", type=float, help="Validation size per fold (count or fraction 0..1).")
    parser.add_argument(
        "--mode", choices=["expanding", "rolling"], default="expanding",
        help="Window mode: expanding training window or rolling fixed-length lookback."
    )
    parser.add_argument("--purge", type=int, default=0, help="Purge window count preceding test set.")
    parser.add_argument("--embargo", type=int, default=0, help="Embargo window count following test set.")
    parser.add_argument("--summary", action="store_true", help="Print fold boundaries summary table.")
    parser.add_argument("--json", action="store_true", help="Output split metadata as JSON.")
    parser.add_argument("--dry-run", action="store_true", help="Validate parameters and display configuration without loading full data.")

    args = parser.parse_args()

    # Determine sample count
    if args.dry_run or (args.input is None and args.n_samples is not None):
        n_samples = args.n_samples if args.n_samples else 1000
    elif args.input:
        if args.dry_run:
            n_samples = 1000
        else:
            try:
                records = load_csv_data(args.input, args.time_col)
                n_samples = len(records)
            except Exception as e:
                sys.stderr.write(f"Error loading input file '{args.input}': {e}\n")
                return 2
    else:
        sys.stderr.write("Error: Either --input <file.csv> or --n-samples <count> must be provided.\n")
        return 2

    train_size = int(args.train_size) if args.train_size and args.train_size >= 1 else args.train_size
    test_size = int(args.test_size) if args.test_size and args.test_size >= 1 else args.test_size

    try:
        splitter = WalkForwardSplitter(
            n_splits=args.n_splits,
            train_size=train_size,
            test_size=test_size,
            mode=args.mode,
            purge_window=args.purge,
            embargo_window=args.embargo,
        )
        folds = splitter.compute_fold_ranges(n_samples)
    except Exception as e:
        sys.stderr.write(f"Configuration Error: {e}\n")
        return 2

    if args.json:
        output_payload = {
            "n_samples": n_samples,
            "n_splits": args.n_splits,
            "mode": args.mode,
            "purge_window": args.purge,
            "embargo_window": args.embargo,
            "dry_run": args.dry_run,
            "folds": folds,
        }
        print(json.dumps(output_payload, indent=2))
    elif args.summary or args.dry_run:
        if args.dry_run:
            print("[DRY-RUN] Walk-Forward Cross-Validation Plan:")
        print(format_summary_table(folds, n_samples))
    else:
        # Default compact output
        print(f"Configured {len(folds)} walk-forward folds across {n_samples} samples (mode: {args.mode}).")
        for f in folds:
            print(f"Fold {f['fold']}: train=[{f['train_start']}:{f['train_end']}] (n={f['train_count']}), "
                  f"purge={f['purge_count']}, test=[{f['test_start']}:{f['test_end']}] (n={f['test_count']}), "
                  f"embargo={f['embargo_count']}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
