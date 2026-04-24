from abc import ABC, abstractmethod
from typing import Any, Dict


class BaseAgent(ABC):
    """Базовый класс для всех ролей."""
    
    def __init__(self, role_name: str, system_prompt: str):
        self.role_name = role_name
        self.system_prompt = system_prompt
    
    @abstractmethod
    async def process(self, task: Dict[str, Any]) -> Dict[str, Any]:
        """Обработка задачи. Возвращает результат."""
        pass
    
