-- Gerege Template Version 27.0
-- eID нэвтрэлтийн COMPLETE хариунд иргэний сертификат (cert.value = DER) болон
-- documentNumber ирдэг. Түүнийг задлан хүчинтэй хугацаа / серийн дугаар / issuer
-- / түлхүүрийн төрлийг хадгална — Profile хуудсанд "Гэрчилгээ" хэсэгт харуулна.
-- Бүх багана nullable тул нууц үгтэй хэрэглэгч болон одоо байгаа мөрүүд эвдрэхгүй.

ALTER TABLE users ADD COLUMN IF NOT EXISTS document_number TEXT;        -- төхөөрөмжийн UUID (eID)
ALTER TABLE users ADD COLUMN IF NOT EXISTS cert_serial     TEXT;        -- сертификатын серийн дугаар (hex)
ALTER TABLE users ADD COLUMN IF NOT EXISTS cert_not_before TIMESTAMPTZ; -- хүчинтэй эхлэх
ALTER TABLE users ADD COLUMN IF NOT EXISTS cert_not_after  TIMESTAMPTZ; -- дуусах
ALTER TABLE users ADD COLUMN IF NOT EXISTS cert_issuer     TEXT;        -- олгогч CA (subject CN)
ALTER TABLE users ADD COLUMN IF NOT EXISTS cert_key_type   TEXT;        -- нийтийн түлхүүрийн алгоритм (ECDSA P-256 г.м.)
