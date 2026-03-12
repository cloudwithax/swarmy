#!/usr/bin/env python3

import argparse
import asyncio
import statistics
import sys
import time

try:
    from acp_sdk import MessageCompletedEvent, MessagePartEvent
    from acp_sdk.client import Client
    from acp_sdk.models import Message, MessagePart
except ImportError as exc:
    raise SystemExit(
        "Missing dependency: acp-sdk. Install it in the active Python environment, for example with `pip install acp-sdk`, "
        "or run this script with the workspace interpreter that already has it installed."
    ) from exc


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Smoke-test and benchmark a live ACP server with the official ACP SDK."
    )
    parser.add_argument(
        "--base-url", default="http://127.0.0.1:8000", help="ACP server base URL."
    )
    parser.add_argument("--agent", default="swarmy", help="ACP agent name.")
    parser.add_argument("--sync-runs", type=int, default=3, help="Number of sync runs.")
    parser.add_argument(
        "--async-runs", type=int, default=3, help="Number of async runs."
    )
    parser.add_argument(
        "--stream-runs", type=int, default=3, help="Number of stream runs."
    )
    parser.add_argument(
        "--concurrency-3-runs",
        type=int,
        default=6,
        help="Total stream runs at concurrency 3.",
    )
    parser.add_argument(
        "--concurrency-5-runs",
        type=int,
        default=10,
        help="Total stream runs at concurrency 5.",
    )
    parser.add_argument(
        "--prompt",
        default="Reply with exactly the single word ok.",
        help="Prompt used for sync, async, and short stream runs.",
    )
    parser.add_argument(
        "--long-prompt",
        default="Count from 1 to 40, one number per line, and nothing else.",
        help="Prompt used to inspect multi-part stream output.",
    )
    return parser.parse_args()


def build_message(content: str) -> Message:
    return Message(parts=[MessagePart(content=content, content_type="text/plain")])


def summarize(values: list[float]) -> tuple[float, float, float]:
    return statistics.mean(values), min(values), max(values)


async def list_agents(client: Client) -> list[str]:
    names: list[str] = []
    async for agent in client.agents():
        names.append(agent.name)
    return names


async def bench_sync(
    client: Client, agent: str, prompt: Message, runs: int
) -> tuple[list[float], list[str]]:
    timings: list[float] = []
    outputs: list[str] = []
    for _ in range(runs):
        start = time.perf_counter()
        run = await client.run_sync(agent=agent, input=[prompt])
        timings.append(time.perf_counter() - start)
        outputs.append(run.output[-1].parts[0].content or "")
    return timings, outputs


async def bench_async(
    client: Client, agent: str, prompt: Message, runs: int
) -> tuple[list[float], list[float], list[int], list[str]]:
    create_times: list[float] = []
    total_times: list[float] = []
    poll_counts: list[int] = []
    statuses: list[str] = []
    for _ in range(runs):
        start = time.perf_counter()
        run = await client.run_async(agent=agent, input=[prompt])
        create_times.append(time.perf_counter() - start)
        polls = 0
        while True:
            polls += 1
            current = await client.run_status(run_id=run.run_id)
            if current.status.is_terminal:
                total_times.append(time.perf_counter() - start)
                poll_counts.append(polls)
                statuses.append(str(current.status))
                break
            await asyncio.sleep(0.05)
    return create_times, total_times, poll_counts, statuses


async def stream_once(
    client: Client, agent: str, prompt: Message
) -> tuple[float | None, float, list[str], bool]:
    start = time.perf_counter()
    first_part: float | None = None
    parts: list[str] = []
    completed = False
    async for event in client.run_stream(agent=agent, input=[prompt]):
        match event:
            case MessagePartEvent(part=part):
                if first_part is None:
                    first_part = time.perf_counter() - start
                parts.append(part.content or "")
            case MessageCompletedEvent():
                completed = True
    total = time.perf_counter() - start
    return first_part, total, parts, completed


async def bench_stream(
    client: Client, agent: str, prompt: Message, runs: int
) -> tuple[list[float], list[float], int]:
    first_parts: list[float] = []
    totals: list[float] = []
    completed = 0
    for _ in range(runs):
        first_part, total, _, is_completed = await stream_once(client, agent, prompt)
        if first_part is not None:
            first_parts.append(first_part)
        totals.append(total)
        completed += int(is_completed)
    return first_parts, totals, completed


async def bench_stream_concurrent(
    base_url: str, agent: str, prompt: Message, total: int, concurrency: int
) -> tuple[float, list[float], list[float], int, list[int]]:
    sem = asyncio.Semaphore(concurrency)

    async def worker() -> tuple[float | None, float, list[str], bool]:
        async with sem:
            async with Client(base_url=base_url) as client:
                return await stream_once(client, agent, prompt)

    start = time.perf_counter()
    results = await asyncio.gather(*[worker() for _ in range(total)])
    wall = time.perf_counter() - start
    first_parts = [
        first_part for first_part, _, _, _ in results if first_part is not None
    ]
    totals = [total_time for _, total_time, _, _ in results]
    completed = sum(1 for _, _, _, is_completed in results if is_completed)
    part_counts = [len(parts) for _, _, parts, _ in results]
    return wall, first_parts, totals, completed, part_counts


async def main() -> int:
    args = parse_args()
    short_prompt = build_message(args.prompt)
    long_prompt = build_message(args.long_prompt)

    async with Client(base_url=args.base_url) as client:
        await client.ping()
        print("agents", await list_agents(client))

        sync_times, outputs = await bench_sync(
            client, args.agent, short_prompt, args.sync_runs
        )
        sync_avg, sync_min, sync_max = summarize(sync_times)
        print(
            f"sync avg={sync_avg:.4f}s min={sync_min:.4f}s max={sync_max:.4f}s outputs={outputs}"
        )

        create_times, total_times, poll_counts, statuses = await bench_async(
            client, args.agent, short_prompt, args.async_runs
        )
        create_avg, _, _ = summarize(create_times)
        total_avg, _, _ = summarize(total_times)
        poll_avg = statistics.mean(poll_counts)
        print(
            f"async create_avg={create_avg:.4f}s total_avg={total_avg:.4f}s poll_avg={poll_avg:.2f} statuses={statuses}"
        )

        first_parts, stream_totals, completed = await bench_stream(
            client, args.agent, short_prompt, args.stream_runs
        )
        first_avg, _, _ = summarize(first_parts)
        stream_avg, _, _ = summarize(stream_totals)
        print(
            f"stream avg_first_part={first_avg:.4f}s avg_total={stream_avg:.4f}s completed={completed}/{args.stream_runs}"
        )

        wall3, first3, total3, completed3, parts3 = await bench_stream_concurrent(
            args.base_url, args.agent, short_prompt, args.concurrency_3_runs, 3
        )
        print(
            f"stream_c3 wall={wall3:.4f}s throughput={args.concurrency_3_runs / wall3:.2f} req/s "
            f"avg_first_part={statistics.mean(first3):.4f}s avg_total={statistics.mean(total3):.4f}s "
            f"avg_parts={statistics.mean(parts3):.2f} completed={completed3}/{args.concurrency_3_runs}"
        )

        wall5, first5, total5, completed5, parts5 = await bench_stream_concurrent(
            args.base_url, args.agent, short_prompt, args.concurrency_5_runs, 5
        )
        print(
            f"stream_c5 wall={wall5:.4f}s throughput={args.concurrency_5_runs / wall5:.2f} req/s "
            f"avg_first_part={statistics.mean(first5):.4f}s avg_total={statistics.mean(total5):.4f}s "
            f"avg_parts={statistics.mean(parts5):.2f} completed={completed5}/{args.concurrency_5_runs}"
        )

        long_first, long_total, long_parts, long_completed = await stream_once(
            client, args.agent, long_prompt
        )
        long_preview = "".join(long_parts)[:200].replace("\n", "\\n")
        print(
            f"long_stream first_part={long_first:.4f}s total={long_total:.4f}s part_count={len(long_parts)} "
            f"completed={long_completed} total_chars={len(''.join(long_parts))} preview={long_preview}"
        )

    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(asyncio.run(main()))
    except KeyboardInterrupt:
        raise SystemExit(130)
