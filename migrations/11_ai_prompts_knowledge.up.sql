-- Gerege Template Version 27.0
-- AI туслах: тохируулдаг prompt давхаргууд + AI-ийн хайдаг мэдлэгийн сан.
--
-- ai_prompts: suurь (base) дүрэм кодод хатуу бичигдсэн — энд зөвхөн
-- 'scope' (хамрах хүрээ) ба 'instructions' (нэмэлт заавар) давхарга
-- хадгалагдаж, админ ажиллаж байх үед нь өөрчилдөг. Шинэ key-г app
-- INSERT хийдэггүй (SetPrompt нь UPDATE-only) тул жагсаалт энд хаалттай.
CREATE TABLE IF NOT EXISTS ai_prompts (
    key        TEXT PRIMARY KEY,
    content    TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ
);

INSERT INTO ai_prompts(key, content) VALUES
    ('scope', 'Чи Gerege платформын албан ёсны туслах. Зөвхөн Gerege платформын үйлчилгээ, бүртгэл, нэвтрэлт, аюулгүй байдал, тохиргоо болон мэдлэгийн санд байгаа сэдвээр тусална.'),
    ('instructions', '')
ON CONFLICT (key) DO NOTHING;

-- ai_knowledge: AI туслахын search_knowledge tool-ийн хайдаг сан. Template
-- хэмжээнд ILIKE хайлт хангалттай; том сан дээр tsvector (full-text) эсвэл
-- pgvector (semantic) руу шилжүүлэхэд репозиторын нэг query солиход л болно.
CREATE TABLE IF NOT EXISTS ai_knowledge (
    id         SERIAL PRIMARY KEY,
    title      TEXT NOT NULL,
    content    TEXT NOT NULL,
    tags       TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

-- Жишээ бичлэгүүд — search_knowledge tool-ийг шууд үзүүлэхэд (проект өөрийн
-- агуулгаар солино).
INSERT INTO ai_knowledge(id, title, content, tags) VALUES
    (1, 'Нууц үг сэргээх', 'Нэвтрэх хуудасны «Нууц үгээ мартсан» холбоосоор имэйлээ оруулахад нэг удаагийн код очно. Кодоо оруулаад 12-оос доошгүй тэмдэгттэй, том/жижиг үсэг, тоо, тусгай тэмдэгт агуулсан шинэ нууц үг тохируулна.', '{нууц үг,сэргээх,password}'),
    (2, 'Бүртгэл идэвхжүүлэх', 'Бүртгүүлсний дараа имэйлээр очсон нэг удаагийн (OTP) кодыг баталгаажуулах хуудсанд оруулснаар бүртгэл идэвхжинэ. Код очоогүй бол дахин илгээх товчийг ашиглана.', '{бүртгэл,otp,идэвхжүүлэх}'),
    (3, 'Эрхийн систем (RBAC)', 'Хэрэглэгч бүр нэг эрхтэй (админ, менежер, хэрэглэгч г.м.). Админ бүх эрхийг автоматаар эзэмшинэ; бусад эрхийн зөвшөөрлүүдийг админ Эрх (RBAC) хэсгээс тохируулна.', '{эрх,rbac,role}')
ON CONFLICT (id) DO NOTHING;

-- Гараар id-тай seed хийсэн тул sequence-ийг гүйцээнэ.
SELECT setval('ai_knowledge_id_seq', GREATEST((SELECT MAX(id) FROM ai_knowledge), 1));
