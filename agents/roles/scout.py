from typing import Any, Dict

from ..tools.llm import generate
from ..tools.search import web_search
from .base import BaseAgent


class ScoutAgent(BaseAgent):
    """Агент-разведчик: ищет информацию в интернете."""
    
    async def process(self, task: Dict[str, Any]) -> Dict[str, Any]:
        query = task.get("query", task.get("text", ""))
        if not query:
            return {"error": "No query provided", "role": self.role_name}
        
        # Выполняем поиск
        search_results = await web_search(query, num_results=7)
        
        # Формируем суммаризацию (опционально)
        if task.get("summarize", True):
            context = "\n\n".join([f"{r['title']}: {r['content']}" for r in search_results])
            summary_prompt = f"Суммируй следующую информацию по запросу '{query}':\n{context}"
            summary = await generate(summary_prompt, system_prompt=self.system_prompt)
            return {
                "role": self.role_name,
                "query": query,
                "results": search_results,
                "summary": summary,
                "sources": [r["url"] for r in search_results]
            }
        else:
            return {
                "role": self.role_name,
                "query": query,
                "results": search_results,
                "sources": [r["url"] for r in search_results]
            }
