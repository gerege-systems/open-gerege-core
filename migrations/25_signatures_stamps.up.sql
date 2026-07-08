-- Гарын үсэг (хувь хүн) ба тамганы дардас (байгууллага) зураг — data-URL текстээр.
ALTER TABLE users ADD COLUMN IF NOT EXISTS signature_image text;

-- Байгууллагын тамганы дардас — улсын бүртгэлийн дугаараар (org_register). Зөвхөн
-- тухайн байгууллагын ADMIN тавьж/устгаж чадна (эрхийг eidmongolia-гаар шалгана).
CREATE TABLE IF NOT EXISTS org_stamps (
    org_register varchar(16)  PRIMARY KEY,
    image        text         NOT NULL,
    uploaded_by  uuid,
    updated_at   timestamptz  NOT NULL DEFAULT now()
);
