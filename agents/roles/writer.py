from typing import Any, Dict

from ..tools.llm import generate
from .base import BaseAgent


class WriterAgent(BaseAgent):
    """Писатель: генерирует пост в блог на основе анализа."""
    
    async def process(self, task: Dict[str, Any]) -> Dict[str, Any]:
        analysis = task.get("analysis", "")
        topic = task.get("topic", "")
        style = task.get("style", "blog_post")  # blog_post, twitter_thread, newsletter
        
        prompt = f"""Напиши {style} на тему "{topic}" на основе следующего анализа:

{analysis}

Требования:
- Заголовок
- Вступление
- Основная часть с подзаголовками
- Заключение
- Призыв к действию (если уместно)
- Длина: примерно 800-1500 слов
"""
        draft = await generate(prompt, system_prompt=self.system_prompt)
        return {
            "role": self.role_name,
            "draft": draft,
            "topic": topic,
            "style": style
        }
        
                
        