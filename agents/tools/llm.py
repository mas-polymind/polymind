import logging
import os
from typing import Any, Dict, List

import httpx
from openai import AsyncOpenAI

# Настройка логирования
logger = logging.getLogger(__name__)

# Конфигурация из переменных окружения
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "ollama").lower()

# Конфигурация Ollama
OLLAMA_HOST = os.getenv("OLLAMA_HOST", "ollama:11434")
# Ensure OLLAMA_HOST has a protocol
if not OLLAMA_HOST.startswith(("http://", "https://")):
    OLLAMA_HOST = f"http://{OLLAMA_HOST}"
OLLAMA_MODEL = os.getenv("OLLAMA_MODEL", "qwen2.5:1.5b")

# Конфигурация OpenRouter
OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY", "")
OPENROUTER_URL = os.getenv("OPENROUTER_URL", "https://openrouter.ai/api/v1")
OPENROUTER_MODEL = os.getenv("OPENROUTER_MODEL", "openrouter/elephant-alpha")
SITE_URL = os.getenv("SITE_URL", "https://ai-agency.example.com")
SITE_NAME = os.getenv("SITE_NAME", "AI Agency System")

# Общие настройки
TEMPERATURE = float(os.getenv("TEMPERATURE", "0.7"))
MAX_TOKENS = int(os.getenv("MAX_TOKENS", "2048"))

# Инициализация клиентов
openrouter_client = None
if LLM_PROVIDER == "openrouter" and OPENROUTER_API_KEY:
    try:
        openrouter_client = AsyncOpenAI(
            base_url=OPENROUTER_URL,
            api_key=OPENROUTER_API_KEY,
            timeout=httpx.Timeout(60.0, connect=10.0)
        )
        logger.info(
            f"OpenRouter client initialized with model: {OPENROUTER_MODEL}"
        )
    except Exception as e:
        logger.error(f"Failed to initialize OpenRouter client: {e}")
        openrouter_client = None


async def generate(
    prompt: str,
    system_prompt: str = "",
    tools: List[Dict] = None
) -> str:
    """
    Универсальная функция генерации текста через выбранный провайдер LLM.
    
    Args:
        prompt: Пользовательский промпт
        system_prompt: Системный промпт (опционально)
        tools: Список инструментов для function calling (опционально)
    
    Returns:
        Сгенерированный текст
    
    Raises:
        ValueError: Если провайдер не поддерживается или конфигурация неверная
        httpx.HTTPError: При ошибках сети или API
    """
    if LLM_PROVIDER == "openrouter":
        return await _generate_openrouter(prompt, system_prompt, tools)
    else:  # ollama по умолчанию
        return await _generate_ollama(prompt, system_prompt, tools)


async def _generate_ollama(
    prompt: str,
    system_prompt: str = "",
    tools: List[Dict] = None
) -> str:
    """Генерация через Ollama API."""
    logger.debug(
        f"Ollama generation: model={OLLAMA_MODEL}, "
        f"prompt_length={len(prompt)}"
    )
    
    # 30 минут для локального Ollama
    async with httpx.AsyncClient(timeout=1800.0) as client:
        payload: Dict[str, Any] = {
            "model": OLLAMA_MODEL,
            "prompt": prompt,
            "system": system_prompt,
            "stream": False,
            "options": {
                "temperature": TEMPERATURE,
                "num_predict": MAX_TOKENS
            }
        }
        
        if tools:
            payload["tools"] = tools
        
        try:
            url = f"{OLLAMA_HOST}/api/generate"
            response = await client.post(url, json=payload)
            response.raise_for_status()
            data = response.json()
            result = data.get("response", "")
            logger.debug(f"Ollama response received, length={len(result)}")
            return result
        except httpx.TimeoutException:
            logger.error("Ollama request timeout")
            raise
        except httpx.HTTPError as e:
            logger.error(f"Ollama HTTP error: {e}")
            raise
        except Exception as e:
            logger.error(f"Unexpected error in Ollama generation: {e}")
            raise


async def _generate_openrouter(
    prompt: str,
    system_prompt: str = "",
    tools: List[Dict] = None
) -> str:
    """Генерация через OpenRouter API."""
    if not openrouter_client:
        raise ValueError(
            "OpenRouter client not initialized. Check OPENROUTER_API_KEY."
        )
    
    logger.debug(
        f"OpenRouter generation: model={OPENROUTER_MODEL}, "
        f"prompt_length={len(prompt)}"
    )
    
    # Подготовка сообщений
    messages = []
    if system_prompt:
        messages.append({"role": "system", "content": system_prompt})
    messages.append({"role": "user", "content": prompt})
    
    # Дополнительные параметры
    extra_headers = {
        "HTTP-Referer": SITE_URL,
        "X-OpenRouter-Title": SITE_NAME,
    }
    
    extra_body = {}
    if tools:
        extra_body["tools"] = tools
    
    try:
        completion = await openrouter_client.chat.completions.create(
            model=OPENROUTER_MODEL,
            messages=messages,
            temperature=TEMPERATURE,
            max_tokens=MAX_TOKENS,
            extra_headers=extra_headers,
            extra_body=extra_body if extra_body else None
        )
        
        result = completion.choices[0].message.content or ""
        logger.debug(f"OpenRouter response received, length={len(result)}")
        return result
    except Exception as e:
        logger.error(f"OpenRouter API error: {e}")
        # Fallback на Ollama если OpenRouter недоступен
        if os.getenv("ENABLE_FALLBACK", "true").lower() == "true":
            logger.warning("Falling back to Ollama due to OpenRouter error")
            return await _generate_ollama(prompt, system_prompt, tools)
        raise


async def generate_with_retry(
    prompt: str, 
    system_prompt: str = "", 
    max_retries: int = 3,
    tools: List[Dict] = None
) -> str:
    """
    Генерация с повторными попытками при ошибках.
    
    Args:
        prompt: Пользовательский промпт
        system_prompt: Системный промпт
        max_retries: Максимальное количество попыток
        tools: Список инструментов
    
    Returns:
        Сгенерированный текст
    """
    last_error = None
    
    for attempt in range(max_retries):
        try:
            return await generate(prompt, system_prompt, tools)
        except Exception as e:
            last_error = e
            wait_time = 2 ** attempt  # Экспоненциальная backoff
            logger.warning(
                f"Attempt {attempt + 1}/{max_retries} failed: {e}. "
                f"Retrying in {wait_time}s"
            )
            
            if attempt < max_retries - 1:
                import asyncio
                await asyncio.sleep(wait_time)
    
    logger.error(
        f"All {max_retries} attempts failed. Last error: {last_error}"
    )
    raise last_error or RuntimeError("Generation failed after all retries")


# Функции для обратной совместимости
async def call_llm(prompt: str, system_prompt: str = "") -> str:
    """Совместимость со старым кодом."""
    return await generate(prompt, system_prompt)


async def call_llm_with_retry(
    prompt: str,
    system_prompt: str = "",
    max_retries: int = 3
) -> str:
    """Совместимость со старым кодом."""
    return await generate_with_retry(prompt, system_prompt, max_retries)