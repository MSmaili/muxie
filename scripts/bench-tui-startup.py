#!/usr/bin/env python3
"""Measure process launch → first FILTER header bytes on a 120×32 PTY.

Build binaries beforehand; pass two for a shuffled A/B comparison. Uses the
current tmux server and frecency state read-only; never sends a navigation key.
This measures warm launch/output latency, not shell overhead or screen painting.
"""

import argparse
import fcntl
import os
from pathlib import Path
import pty
import random
import select
import statistics
import struct
import subprocess
import termios
import time


def first_header(binary):
    master, slave = pty.openpty()
    process = None
    try:
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 32, 120, 0, 0))
        env = dict(os.environ, TERM="xterm-256color")
        start = time.perf_counter()
        process = subprocess.Popen(
            [str(binary)], stdin=slave, stdout=slave, stderr=slave, env=env,
        )
        deadline = start + 5
        output = b""
        while b"FILTER" not in output:
            remaining = deadline - time.perf_counter()
            if remaining <= 0 or not select.select([master], [], [], remaining)[0]:
                raise RuntimeError(f"{binary}: no FILTER header within 5 seconds")
            chunk = os.read(master, 65536)
            if not chunk:
                raise RuntimeError(f"{binary}: closed PTY before FILTER header")
            output = (output + chunk)[-65536:]
        return (time.perf_counter() - start) * 1000
    finally:
        # Discard this isolated PTY after the timed event; shutdown isn't timed.
        if process is not None:
            if process.poll() is None:
                process.kill()
            process.wait(timeout=2)
        os.close(slave)
        os.close(master)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binaries", nargs="+", type=Path)
    parser.add_argument("--samples", type=int, default=50)
    args = parser.parse_args()
    if args.samples < 1:
        parser.error("--samples must be positive")
    binaries = [path.resolve() for path in args.binaries]
    for binary in binaries:
        if not binary.is_file() or not os.access(binary, os.X_OK):
            parser.error(f"not an executable: {binary}")
        print(f"{binary.name}: first measured launch {first_header(binary):.2f} ms")
        for _ in range(5):
            first_header(binary)

    order = list(range(len(binaries))) * args.samples
    random.Random(42).shuffle(order)
    results = [[] for _ in binaries]
    for index in order:
        results[index].append(first_header(binaries[index]))
    for binary, samples in zip(binaries, results):
        samples.sort()
        p95 = samples[round((len(samples) - 1) * 0.95)]
        print(
            f"{binary.name}: n={len(samples)} min={samples[0]:.2f} "
            f"median={statistics.median(samples):.2f} p95={p95:.2f} "
            f"max={samples[-1]:.2f} ms"
        )


if __name__ == "__main__":
    main()
