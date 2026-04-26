import asyncio
import json
import os
import re
import signal
import traceback
from typing import Any, Dict

import aio_pika
import httpx
from aio_pika.abc import DeliveryMode
from ddgs import DDGS
# Импорт универсального LLM клиента
from tools.llm import call_llm_with_retry

# ---------- Конфигурация ----------
ROLE = os.getenv("AGENT_ROLE", "scout")
OLLAMA_HOST = os.getenv("OLLAMA_HOST", "ollama:11434")
SEARXNG_URL = os.getenv("SEARXNG_URL", "http://searxng:8080")
RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
MODEL_NAME = os.getenv("MODEL_NAME", "qwen2.5:1.5b")
TEMPERATURE = float(os.getenv("TEMPERATURE", "0.7"))

# Конфигурация LLM провайдера
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "ollama").lower()
OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY", "")
OPENROUTER_URL = os.getenv("OPENROUTER_URL",
                           "https://openrouter.ai/api/v1")
OPENROUTER_MODEL = os.getenv("OPENROUTER_MODEL",
                             "meta-llama/llama-3.2-3b-instruct:free")
FALLBACK_TO_OLLAMA = os.getenv("FALLBACK_TO_OLLAMA",
                               "true").lower() == "true"

INPUT_QUEUE = f"agent.{ROLE}"
OUTPUT_QUEUE = "task.result"

# Логирование выбранного провайдера
print(f"[{ROLE}] LLM провайдер: {LLM_PROVIDER}")
if LLM_PROVIDER == "openrouter":
    print(f"[{ROLE}] OpenRouter модель: {OPENROUTER_MODEL}")
    if not OPENROUTER_API_KEY:
        print(f"[{ROLE}] ВНИМАНИЕ: OPENROUTER_API_KEY не установлен!")
else:
    print(f"[{ROLE}] Ollama хост: {OLLAMA_HOST}, модель: {MODEL_NAME}")

# ---------- Поиск через SearXNG ----------
async def search_web_searxng(query: str) -> str:
    """Поиск через SearXNG."""
    url = f"{SEARXNG_URL}/search"
    params = {"q": query, "format": "json", "language": "ru-RU"}
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.get(url, params=params)
            text = response.text
            
            # Очистка JSON (SearXNG иногда возвращает с префиксом)
            start = text.find('{')
            if start == -1:
                return ""
            json_part = text[start:]
            end = json_part.rfind('}') + 1
            if end == 0:
                return ""
            json_part = json_part[:end]
            
            data = json.loads(json_part)
            results = [res.get('content', '') for res in data.get('results', [])[:5]]
            return "\n---\n".join(results) if results else ""
    except Exception as e:
        print(f"[{ROLE}] Ошибка SearXNG: {e}")
        return ""

async def search_web_ddgs(query: str) -> str:
    """Резервный поиск через DuckDuckGo."""
    try:
        with DDGS() as ddgs:
            results = [r['body'] for r in ddgs.text(query, max_results=3)]
            return "\n".join(results) if results else ""
    except Exception as e:
        print(f"[{ROLE}] Ошибка DuckDuckGo: {e}")
        return ""

async def search_web(query: str) -> str:
    """Объединённый поиск: сначала SearXNG, потом DuckDuckGo."""
    result = await search_web_searxng(query)
    if result:
        print(f"[{ROLE}] Найдено через SearXNG")
        return result
    
    print(f"[{ROLE}] SearXNG не дал результатов, пробую DuckDuckGo")
    return await search_web_ddgs(query)

# ---------- Обработчики по ролям ----------
async def process_scout(task: Dict) -> Dict[str, Any]:
    """Разведчик: поиск информации в интернете."""
    if not task.get("query"):
        return {"summary": "Нет запроса для поиска", "raw_data": "", "sources": []}
    
    print(f"[scout] Поиск: {task['query']}")
    raw_data = await search_web(task["query"])
    
    if not raw_data:
        return {"summary": "Ничего не найдено", "raw_data": "", "sources": []}
    
    summary_prompt = f"""Суммируй следующую информацию по запросу '{task['query']}':

{raw_data}

Выдели основные факты и ключевые моменты. Ответ должен быть кратким и информативным."""
    
    summary = await call_llm_with_retry(summary_prompt, "Ты — разведчик. Кратко суммируй найденную информацию.")
    
    return {"summary": summary, "raw_data": raw_data, "sources": []}

async def process_analyst(task: Dict) -> Dict[str, Any]:
    """Аналитик: структурирование и анализ данных."""
    if not task.get("data"):
        return {"analysis": "Нет данных для анализа"}
    
    print(f"[analyst] Анализ: {task.get('query')}")
    data = task["data"]
    
    # Если data - словарь, извлекаем summary
    if isinstance(data, dict):
        data = data.get("summary", str(data))
    
    prompt = f"""Проанализируй следующие данные по запросу "{task.get('query')}":

{data}

Выдели:
1. Ключевые факты
2. Основные выводы
3. Противоречия (если есть)
4. Рекомендации для написания блога

Ответ должен быть структурированным и информативным."""
    
    analysis = await call_llm_with_retry(prompt, "Ты — аналитик. Структурируй ответ.")
    return {"analysis": analysis}

async def process_writer(task: Dict) -> Dict[str, Any]:
    """Писатель: создание черновика поста или доработка по рецензии."""
    print(f"[writer] Обработка: {task.get('topic')}")
    
    if not task.get("analysis"):
        return {"error": "Нет анализа для написания поста"}
    
    iteration = task.get("iteration", 0)
    previous_score = task.get("previous_score")
    
    # Режим доработки (есть рецензия и черновик)
    if task.get("review") and task.get("draft"):
        print(f"[writer] Режим доработки с учётом рецензии (итерация {iteration})")
        
        iteration_context = ""
        if iteration > 0:
            iteration_context = f"Это итерация {iteration} доработки поста. "
            if previous_score:
                iteration_context += f"Предыдущая оценка: {previous_score}/10. "
            iteration_context += "Учти все предыдущие замечания и сделай текст лучше.\n\n"
        
        prompt = f"""{iteration_context}Ты — писатель блога. Перепиши пост с учётом рецензии.

Тема: "{task['topic']}"
Анализ (исходные данные): {task['analysis']}

Предыдущий черновик:
{task['draft']}

Рецензия критика (замечания):
{task['review']}

ВАЖНО: Не копируй структуру анализа (не используй разделы "Ключевые факты", "Основные выводы" и т.д.).
Создай плавный, связный текст в формате статьи для блога.

КРИТИЧЕСКИ ВАЖНО:
1. Устрани все замечания из рецензии
2. Улучши структуру и читабельность
3. Сохрани заголовок, вступление, основную часть с подзаголовками и заключение
4. Преобразуй текст в формат статьи для блога (не аналитический отчет)
5. НИКОГДА не добавляй мета-комментарии, объяснения или оценки текста
6. НИКОГДА не используй фразы типа: "Таким образом...", "В итоге...", "Это делает текст...", "Текст будет более..."
7. Просто напиши статью. Никаких дополнительных слов.

Ответь ТОЛЬКО готовым текстом поста. Никаких комментариев, объяснений, мета-рассуждений."""
        system_prompt = "Ты — профессиональный писатель блога. Учитывай критику и улучшай текст. Преобразуй аналитическую структуру в увлекательную статью для блога. НИКОГДА не добавляй мета-комментарии или объяснения своих действий. Пиши только текст статьи."
    else:
        print(f"[writer] Режим создания нового черновика (итерация {iteration})")
        prompt = f"""Напиши пост в блог на тему "{task['topic']}" на основе следующего анализа.
ВАЖНО: Не копируй структуру анализа (не используй разделы "Ключевые факты", "Основные выводы" и т.д.).
Вместо этого создай плавный, связный текст в формате статьи для блога.

Анализ для справки:
{task['analysis']}

Требования к статье:
1. Заголовок (интересный, привлекающий внимание)
2. Вступление (представь тему, заинтересуй читателя)
3. Основная часть (раздели на логические разделы с подзаголовками, но не используй маркированные списки как в анализе)
4. Заключение (подведи итоги, сделай выводы)
5. Длина: 500-1000 слов
6. Стиль: живой, engaging, для широкой аудитории

ЗАПРЕЩЕНО:
- Разделы "Ключевые факты", "Основные выводы", "Противоречия", "Рекомендации"
- Маркированные списки из анализа
- Технические пометки
- Любые мета-комментарии о структуре или качестве текста
- Фразы: "Таким образом...", "В результате...", "Это позволяет...", "Текст становится..."
- Объяснения своих действий или оценок текста

Ответь ТОЛЬКО готовым текстом статьи в формате блога. Никаких дополнительных слов."""
        system_prompt = "Ты — профессиональный писатель научно-популярного блога. Преобразуй структурированный анализ в увлекательную статью для широкой аудитории. Не копируй структуру анализа, создавай плавный повествовательный текст. НИКОГДА не добавляй мета-комментарии или объяснения."
    
    draft = await call_llm_with_retry(prompt, system_prompt)
    
    if not draft or not draft.strip():
        # Если черновик пустой, используем текст запроса как черновик
        fallback_draft = f"Пост на тему: {task.get('topic', 'Без темы')}. Текст не был сгенерирован, но работа продолжается."
        print(f"[writer] Черновик пустой, используем fallback: {fallback_draft[:100]}...")
        return {"draft": fallback_draft}
    
    # Пост-обработка: удаление мета-комментариев
    import re

    # Удаляем типичные мета-фразы
    meta_phrases = [
        "Таким образом,",
        "В итоге,",
        "Это делает текст",
        "Текст будет более",
        "В результате,",
        "Позволяет сделать текст",
        "Следовательно,",
        "Итак,",
        "Как видно,",
        "Очевидно, что",
    ]
    
    for phrase in meta_phrases:
        # Ищем фразы в начале предложений (с игнорированием регистра)
        pattern = rf'\n{re.escape(phrase)}.*?\n'
        draft = re.sub(pattern, '\n', draft, flags=re.IGNORECASE)
        pattern_start = rf'^{re.escape(phrase)}.*?\n'
        draft = re.sub(pattern_start, '', draft, flags=re.IGNORECASE)
    
    # Удаляем пустые строки, которые могли образоваться
    draft = '\n'.join([line for line in draft.split('\n') if line.strip()])
    
    # Если после пост-обработки черновик стал пустым, также используем fallback
    if not draft.strip():
        fallback_draft = f"Пост на тему: {task.get('topic', 'Без темы')}. Текст был удален при пост-обработке."
        print(f"[writer] После пост-обработки черновик пустой, используем fallback")
        return {"draft": fallback_draft}
    
    return {"draft": draft}

async def process_critic(task: Dict) -> Dict[str, Any]:
    """Критик: оценка черновика и рецензия."""
    print(f"[critic] Рецензирование: {task.get('topic')}")
    
    if not task.get("draft"):
        return {"review": "Нет черновика для рецензии", "score": 0.0}
    
    prompt = f"""Ты — строгий редактор. Оцени черновик поста на тему "{task['topic']}":

{task['draft']}

Выставь оценки (1-10) по каждому критерию:
- Фактическая точность (насколько текст соответствует реальности)
- Структура и логика (последовательность изложения)
- Читабельность (простота восприятия)
- Уникальность и оригинальность
- Грамматика и стиль

После этого напиши 3-5 конкретных предложений по улучшению.

В самом конце напиши строку: SCORE: X.X (среднее арифметическое пяти оценок)."""
    
    review = await call_llm_with_retry(prompt, "Ты — строгий, но конструктивный редактор.")
    
    # Извлечение оценки
    score = 5.0
    match = re.search(r'SCORE:\s*(\d+(?:\.\d+)?)', review)
    if match:
        score = float(match.group(1))
    else:
        # Альтернативный поиск
        alt_match = re.search(r'(\d+(?:\.\d+)?)\s*\/\s*10', review)
        if alt_match:
            score = float(alt_match.group(1))
    
    return {"review": review, "score": score}

# ---------- Обработчик сообщений из RabbitMQ ----------
async def process_message(message: aio_pika.IncomingMessage):
    # ignore_processed=True предотвратит ChannelInvalidStateError при закрытии
    async with message.process(ignore_processed=True):
        try:
            body = json.loads(message.body.decode())
            task_id = body.get("task_id")
            print(f"[{ROLE}] Получена задача {task_id}")
            
            # Вызов соответствующей функции
            if ROLE == "scout":
                result = await process_scout(body)
            elif ROLE == "analyst":
                result = await process_analyst(body)
            elif ROLE == "writer":
                result = await process_writer(body)
            elif ROLE == "critic":
                result = await process_critic(body)
            else:
                result = {"error": f"Unknown role {ROLE}"}
            
            success = "error" not in result
            output = {
                "task_id": task_id,
                "step": ROLE,
                "result": result,
                "success": success
            }
            
            # Отправка результата обратно в оркестратор
############
            try:
                # В 9.x мы создаем объект сообщения и передаем его в publish
                message_to_send = aio_pika.Message(
                    body=json.dumps(output).encode(),
                    delivery_mode=DeliveryMode.PERSISTENT, # Используем импортированный enum
                    content_type='application/json'
                )
                
                # Используем прямой вызов публикации через канал
                await message.channel.basic_publish(
                    exchange='',
                    routing_key=OUTPUT_QUEUE,
                    body=message_to_send.body,
                    properties=message_to_send.properties
                )
                
                status = "успешно" if success else "с ошибкой"
                print(f"[{ROLE}] Задача {task_id} завершена {status}")
            except Exception as e:
                print(f"[{ROLE}] Ошибка при отправке: {e}")
                traceback.print_exc()


#####
            
        except asyncio.CancelledError:
            # Это происходит при остановке приложения
            print(f"[{ROLE}] Задача была прервана сигналом остановки")
            raise # Позволяем asyncio корректно свернуть задачу
        except Exception as e:
            print(f"[{ROLE}] Критическая ошибка: {e}")
            traceback.print_exc()

# ---------- Запуск агента ----------
async def main():
    """Основная функция запуска агента."""
    # Подключение к RabbitMQ
    connection = await aio_pika.connect_robust(RABBITMQ_URL)
    
    async with connection:
        channel = await connection.channel()
        
        # Настройка QoS (рекомендуется): агент берет 1 задачу за раз
        await channel.set_qos(prefetch_count=1)
        
        # Объявление очередей
        input_queue = await channel.declare_queue(INPUT_QUEUE, durable=True)
        await channel.declare_queue(OUTPUT_QUEUE, durable=True)
        
        print(f"[{ROLE}] Агент запущен, слушаю очередь '{INPUT_QUEUE}'")
        print(f"[{ROLE}] Модель: {MODEL_NAME}, температура: {TEMPERATURE}")
        
        # 1. Начинаем слушать и сохраняем consumer_tag
        consumer_tag = await input_queue.consume(process_message)
        
        # Событие для ожидания сигнала остановки
        stop_event = asyncio.Event()
        
        def signal_handler():
            print(f"\n[{ROLE}] Получен сигнал завершения. Останавливаю потребление...")
            stop_event.set()
        
        # Регистрируем обработчики сигналов
        loop = asyncio.get_running_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            loop.add_signal_handler(sig, signal_handler)
        
        # 2. Ждем, пока сработает сигнал (Ctrl+C или остановка контейнера)
        await stop_event.wait()
        
        print(f"[{ROLE}] Получен сигнал остановки. Прекращаю прием новых задач...")
        await input_queue.cancel(consumer_tag)

        # Получаем список всех запущенных задач, кроме текущей (main)
        tasks = [t for t in asyncio.all_tasks() if t != asyncio.current_task()]
        
        if tasks:
            print(f"[{ROLE}] Ожидаю завершения текущих генераций (активно задач: {len(tasks)})...")
            # Ждем завершения всех задач. Timeout можно поставить хоть на 600 секунд (10 минут)
            done, pending = await asyncio.wait(tasks, timeout=600) 
            
            if pending:
                print(f"[{ROLE}] Предупреждение: {len(pending)} задач не завершились вовремя и будут убиты.")
            else:
                print(f"[{ROLE}] Все задачи успешно завершены.")
        
    # Выход из блока 'async with connection' произойдет только здесь
    print(f"[{ROLE}] Соединение с RabbitMQ закрыто. Агент остановлен.")

if __name__ == "__main__":
    asyncio.run(main())
        