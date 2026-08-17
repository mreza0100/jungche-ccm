from .server import serve


def main():
    """MCP Fetch Server - HTTP fetching functionality for MCP"""
    import argparse
    import asyncio
    import os

    parser = argparse.ArgumentParser(
        description="give a model the ability to make web requests"
    )
    parser.add_argument("--user-agent", type=str, help="Custom User-Agent string")
    parser.add_argument(
        "--ignore-robots-txt",
        action="store_true",
        default=True,
        help="No-op, kept for compatibility: robots.txt is never enforced — the server always fetches.",
    )
    parser.add_argument("--proxy-url", type=str, help="Proxy URL to use for requests")
    parser.add_argument(
        "--transport", choices=("stdio", "http"), default="stdio",
        help="stdio (default, local MCP client) or http (Streamable HTTP, for remote clients "
             "such as a claude.ai custom connector)",
    )
    parser.add_argument("--host", default="127.0.0.1", help="http transport: bind address")
    parser.add_argument("--port", type=int, default=8081, help="http transport: bind port")
    parser.add_argument(
        "--public-url", type=str, default=None,
        help="http transport: externally visible origin (e.g. https://host.example.ts.net). Must "
             "match the URL entered in the client — OAuth resource metadata is compared exactly. "
             "Defaults to http://<host>:<port>.",
    )
    parser.add_argument(
        "--internal-host", default="127.0.0.1",
        help="http transport: bind address for the unauthenticated internal gateway",
    )
    parser.add_argument(
        "--internal-port", type=int, default=8082,
        help="http transport: port for the unauthenticated internal gateway onto the same "
             "harvester (0 disables it). Only started when the external gateway has credentials.",
    )
    parser.add_argument(
        "--allow-unauthenticated", action="store_true",
        help="http transport: permit binding an unauthenticated gateway to a non-loopback address",
    )

    args = parser.parse_args()

    if args.transport == "stdio":
        asyncio.run(serve(args.user_agent, args.ignore_robots_txt, args.proxy_url))
        return

    from .remote import run_http

    run_http(
        args.public_url or f"http://{args.host}:{args.port}",
        args.host,
        args.port,
        internal_host=args.internal_host,
        internal_port=args.internal_port,
        allow_unauthenticated_bind=args.allow_unauthenticated,
        user_agent=args.user_agent,
        proxy_url=args.proxy_url,
        passphrase=os.environ.get("HARVESTER_AUTH_PASSPHRASE") or None,
        static_token=os.environ.get("HARVESTER_STATIC_TOKEN") or None,
    )


if __name__ == "__main__":
    main()
