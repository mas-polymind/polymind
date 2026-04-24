from typing import Any, Dict

from ..tools.llm import generate
from .base import BaseAgent


class CriticAgent(BaseAgent):
    """Критик: рецензирует черновик, даёт оценки и предложения."""
    
    async def process(self, task: Dict[str, Any]) -> Dict[str, Any]:
        draft = task.get("draft", "")
        topic = task.get("topic", "")
        
        prompt = f"""Оцени следующий черновик поста на тему "{topic}":

{draft}

Дай оценку по шкале 1-10 для каждого критерия:
- Фактическая точность
- Структура и логика
- Читабельность
- Уникальность
- Грамматика и стиль

Также перечисли конкретные предложения по улучшению (3-5 пунктов).
Итоговая рекомендация: принять / доработать / отклонить.
"""
        review = await generate(prompt, system_prompt=self.system_prompt)
        return {
            "role": self.role_name,
            "review": review,
            "score": 0  # можно распарсить из review, но для простоты пока оставим
        }
        
                
        