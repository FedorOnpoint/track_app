INSERT INTO categories (slug, title, description, sort_order) VALUES
('go-core', 'Go Core', 'Язык, типы, интерфейсы и модель исполнения Go.', 10),
('concurrency', 'Конкурентность', 'Горутины, каналы и синхронизация.', 20),
('http-api', 'HTTP и API', 'Надёжные сетевые сервисы на Go.', 30)
ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description, sort_order = EXCLUDED.sort_order;

INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes)
SELECT id, 'interfaces', 'Интерфейсы в Go', 'Неявная реализация, nil-интерфейсы и практические границы абстракций.', 'intermediate', 10, 25 FROM categories WHERE slug = 'go-core'
ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title, summary = EXCLUDED.summary, difficulty = EXCLUDED.difficulty, estimated_minutes = EXCLUDED.estimated_minutes;
INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes)
SELECT id, 'goroutines', 'Горутины и планировщик', 'Жизненный цикл горутин, GMP-модель и способы избежать утечек.', 'intermediate', 10, 35 FROM categories WHERE slug = 'concurrency'
ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title, summary = EXCLUDED.summary, difficulty = EXCLUDED.difficulty, estimated_minutes = EXCLUDED.estimated_minutes;
INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes)
SELECT id, 'context', 'Context: отмена и дедлайны', 'Передача отмены, дедлайнов и request-scoped данных через границы API.', 'basic', 20, 20 FROM categories WHERE slug = 'concurrency'
ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title, summary = EXCLUDED.summary, difficulty = EXCLUDED.difficulty, estimated_minutes = EXCLUDED.estimated_minutes;
INSERT INTO topics (category_id, slug, title, summary, difficulty, sort_order, estimated_minutes)
SELECT id, 'graceful-shutdown', 'Graceful shutdown HTTP-сервера', 'Корректное завершение запросов и освобождение ресурсов при остановке процесса.', 'advanced', 10, 30 FROM categories WHERE slug = 'http-api'
ON CONFLICT (slug) DO UPDATE SET title = EXCLUDED.title, summary = EXCLUDED.summary, difficulty = EXCLUDED.difficulty, estimated_minutes = EXCLUDED.estimated_minutes;

INSERT INTO topic_sections (topic_id, heading, body_markdown, sort_order)
SELECT id, 'Зачем нужны горутины', 'Горутина — лёгкая функция, которую независимо планирует runtime Go. Она не равна потоку ОС и требует явного управления временем жизни.', 10 FROM topics WHERE slug = 'goroutines'
ON CONFLICT (topic_id, sort_order) DO UPDATE SET heading = EXCLUDED.heading, body_markdown = EXCLUDED.body_markdown;
INSERT INTO topic_sections (topic_id, heading, body_markdown, sort_order)
SELECT id, 'Главный риск', 'Каждая запущенная горутина должна иметь понятное условие завершения. Для отмены долгих операций обычно передают `context.Context`.', 20 FROM topics WHERE slug = 'goroutines'
ON CONFLICT (topic_id, sort_order) DO UPDATE SET heading = EXCLUDED.heading, body_markdown = EXCLUDED.body_markdown;

INSERT INTO checklist_items (topic_id, stable_key, text, sort_order)
SELECT id, 'explain-gmp', 'Могу объяснить роли G, M и P в планировщике.', 10 FROM topics WHERE slug = 'goroutines'
ON CONFLICT (topic_id, stable_key) DO UPDATE SET text = EXCLUDED.text, sort_order = EXCLUDED.sort_order;
INSERT INTO checklist_items (topic_id, stable_key, text, sort_order)
SELECT id, 'spot-leak', 'Умею найти горутину без пути завершения.', 20 FROM topics WHERE slug = 'goroutines'
ON CONFLICT (topic_id, stable_key) DO UPDATE SET text = EXCLUDED.text, sort_order = EXCLUDED.sort_order;

INSERT INTO interview_questions (topic_id, stable_key, question, short_answer, detailed_answer, pitfalls, sort_order)
SELECT id, 'goroutine-vs-thread', 'Чем горутина отличается от потока ОС?', 'Горутина легче, имеет растущий стек и планируется Go runtime поверх потоков ОС.', 'Runtime мультиплексирует множество горутин на ограниченное число потоков и может перепланировать их в точках safe point.', 'Не утверждать, что горутина всегда занимает отдельный поток или что переключение бесплатно.', 10 FROM topics WHERE slug = 'goroutines'
ON CONFLICT (topic_id, stable_key) DO UPDATE SET question = EXCLUDED.question, short_answer = EXCLUDED.short_answer, detailed_answer = EXCLUDED.detailed_answer, pitfalls = EXCLUDED.pitfalls;

INSERT INTO follow_up_questions (interview_question_id, question, expected_answer, sort_order)
SELECT id, 'Что произойдёт при блокирующем syscall?', 'Runtime может передать P другому M, чтобы остальные горутины продолжили исполняться.', 10 FROM interview_questions WHERE stable_key = 'goroutine-vs-thread'
ON CONFLICT (interview_question_id, sort_order) DO UPDATE SET question = EXCLUDED.question, expected_answer = EXCLUDED.expected_answer;

INSERT INTO resources (topic_id, title, url, resource_type, source_name, sort_order)
SELECT id, 'Go Concurrency Patterns', 'https://go.dev/blog/pipelines', 'documentation', 'The Go Blog', 10 FROM topics WHERE slug = 'goroutines'
ON CONFLICT (topic_id, sort_order) DO UPDATE SET title = EXCLUDED.title, url = EXCLUDED.url, resource_type = EXCLUDED.resource_type, source_name = EXCLUDED.source_name;

INSERT INTO practice_recommendations (topic_id, title, description, expected_outcome, sort_order)
SELECT id, 'Остановить pipeline', 'Соберите pipeline из трёх стадий и добавьте отмену без зависших горутин.', 'Все стадии завершаются после отмены, тест проходит с `-race`.', 10 FROM topics WHERE slug = 'goroutines'
ON CONFLICT (topic_id, sort_order) DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description, expected_outcome = EXCLUDED.expected_outcome;
