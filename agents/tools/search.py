import os
from typing import Dict, List

import httpx

SEARXNG_URL = os.getenv("SEARXNG_URL", "http://searxng:8080")

async def web_search(query: str, num_results: int = 5) -> List[Dict]:
    """Поиск через SearXNG."""
    async with httpx.AsyncClient() as client:
        params = {
            "q": query,
            "format": "json",
            "categories": "general",
            "engines": "google,bing,duckduckgo",
            "limit": num_results
        }
        response = await client.get(f"{SEARXNG_URL}/search", params=params)
        response.raise_for_status()
        data = response.json()
        results = []
        for res in data.get("results", [])[:num_results]:
            results.append({
                "title": res.get("title"),
                "url": res.get("url"),
                "content": res.get("content", "")
            })
        return results
    