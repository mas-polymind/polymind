from typing import Any, Dict

from ..tools.llm import generate
from .base import BaseAgent


class AnalystAgent(BaseAgent):
    """Аналитик: обрабатывает найденную информацию, выделяет факты."""
    
    async def process(self, task: Dict[str, Any]) -> Dict[str, Any]:
        data = task.get("data", "")
        query = task.get("query", "")
        
        if not data:
            return {"error": "No data to analyze", "role": self.role_name}
        
        prompt = f"""Проанализируй следующую информацию по запросу "{query}":
        
{data}

Выдели:
1. Ключевые факты (списком)
2. Основные выводы
3. Противоречия (если есть)
4. Рекомендации для дальнейшего исследования
"""
        analysis = await generate(prompt, system_prompt=self.system_prompt)
        return {
            "role": self.role_name,
            "analysis": analysis,
            "original_query": query
        }
        
        
                
        