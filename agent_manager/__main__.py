"""Allow running the agent manager as `python -m agent_manager`."""

import asyncio
import logging
import os

from server import serve

if __name__ == "__main__":
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    port = int(os.environ.get("AGENT_MANAGER_PORT", "50051"))
    asyncio.run(serve(port))
