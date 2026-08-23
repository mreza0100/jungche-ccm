"""Route-guard seam test — runs with NO browser, NO patchright (the seam exists
so CI can pin the R4 CRITICAL: a redirect/subresource ask that Go denies must
ABORT the request, never let Chrome connect).

Run: python3 browser_route_guard_test.py
"""

import asyncio
import sys
import types

from browser import browser_route_guard


class FakeRoute:
    def __init__(self, url):
        self.request = types.SimpleNamespace(url=url)
        self.abort_reason = None
        self.continued = False

    async def abort(self, reason="blocked"):
        self.abort_reason = reason

    async def continue_(self):
        self.continued = True


def run(coro):
    return asyncio.new_event_loop().run_until_complete(coro)


def test_denied_ask_aborts_before_connecting():
    route = FakeRoute("http://169.254.169.254/latest/meta-data/")

    async def deny(url):
        return False, "refusing private/internal host 169.254.169.254"

    guard = browser_route_guard(deny)
    run(guard(route))
    assert route.abort_reason == "blocked", f"expected abort('blocked'), got {route.abort_reason!r}"
    assert not route.continued, "denied route was continued() — Chrome would have connected"


def test_allowed_ask_continues():
    route = FakeRoute("https://example.test/article")

    async def allow(url):
        return True, ""

    guard = browser_route_guard(allow)
    run(guard(route))
    assert route.continued, "allowed route was not continued()"
    assert route.abort_reason is None


def test_raising_ask_aborts_never_continues():
    # B2: the ask channel can die mid-guard (Go closed stdin, malformed
    # reply). A raised handler left Playwright's default in charge — which
    # may CONTINUE the request. The guard must abort on its own.
    route = FakeRoute("http://169.254.169.254/latest/meta-data/")

    async def broken(url):
        raise RuntimeError("go closed the ask channel mid-guard")

    guard = browser_route_guard(broken)
    run(guard(route))
    assert route.abort_reason == "guard-error", (
        f"expected abort('guard-error'), got {route.abort_reason!r}"
    )
    assert not route.continued, "a raising ask must never let the request through"


def test_serialized_ask_never_overlaps():
    # Playwright dispatches route handlers CONCURRENTLY (measured live on
    # patchright 1.62.1). The stdin/stdout ask protocol has no correlation
    # id, so overlapping exchanges could let an ALLOW meant for one URL
    # answer another handler's ask — a fail-open SSRF race.
    from browser import serialized_ask

    state = {"inflight": 0, "max": 0}

    async def slow_ask(url):
        state["inflight"] += 1
        state["max"] = max(state["max"], state["inflight"])
        await asyncio.sleep(0.01)
        state["inflight"] -= 1
        return True, ""

    guarded = serialized_ask(slow_ask)

    async def fire_all():
        await asyncio.gather(*(guarded(f"u{i}") for i in range(5)))

    run(fire_all())
    assert state["max"] == 1, (
        f"asks overlapped (max in-flight {state['max']}) — the protocol raced"
    )


if __name__ == "__main__":
    failures = 0
    for name, case in sorted(globals().items()):
        if name.startswith("test_") and callable(case):
            try:
                case()
            except AssertionError as e:
                print(f"FAIL {name}: {e}", file=sys.stderr)
                failures += 1
            else:
                print(f"ok {name}")
    sys.exit(1 if failures else 0)
