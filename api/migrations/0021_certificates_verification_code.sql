-- Kod weryfikacyjny zaświadczenia (certificates.verification_code).
--
-- Platforma e-learningowa publikuje stronę "sprawdź zaświadczenie" pod adresem z kodem.
-- Kod:
--   - 12 znaków z alfabetu 23456789ABCDEFGHJKLMNPQRSTUVWXYZ (bez 0, O, 1, I, l),
--     czyli 60 bitów losowości - czytelny przy przepisywaniu z papieru,
--   - losowany z generatora kryptograficznego PostgreSQL (gen_random_uuid korzysta
--     z pg_strong_random), niezależny od numeru rejestru i id,
--   - nadawany przez DEFAULT kolumny, więc dostaje go każde zaświadczenie, niezależnie
--     od ścieżki: API, aplikacja webowa, generowanie z dziennika czy ręczny INSERT,
--   - unikalny (indeks unikalny) i niezmienny (wyzwalacz odrzuca UPDATE kodu).
--
-- Wpływ na istniejące dane: każde istniejące zaświadczenie (także usunięte) dostaje
-- losowy kod w kroku 3. Migracja przepisuje wszystkie wiersze tabeli certificates
-- i trzyma na niej blokadę do końca transakcji (na kopii bazy: ~16 tys. wierszy, ok. 1 s). Pozostałe kolumny nie są zmieniane.
-- Wymaga PostgreSQL 13+ (wbudowane gen_random_uuid).
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

-- 1. Generator. UUID v4 ma 122 losowe bity; stałe bity wersji i wariantu siedzą w bajtach
--    6 i 8, więc bierzemy bajty 0-5 i 9-14. Każdy bajt jest równomierny na 0-255, a 256
--    dzieli się przez 32, więc byte % 32 wybiera znak bez obciążenia.
CREATE FUNCTION generate_certificate_verification_code() RETURNS text
    LANGUAGE plpgsql
    VOLATILE
AS $$
DECLARE
    alphabet constant text := '23456789ABCDEFGHJKLMNPQRSTUVWXYZ';
    positions constant int[] := ARRAY[0, 1, 2, 3, 4, 5, 9, 10, 11, 12, 13, 14];
    random_bytes bytea;
    code text;
    pos int;
BEGIN
    LOOP
        random_bytes := uuid_send(gen_random_uuid());
        code := '';
        FOREACH pos IN ARRAY positions LOOP
            code := code || substr(alphabet, get_byte(random_bytes, pos) % 32 + 1, 1);
        END LOOP;
        -- Kolizja przy 60 bitach jest praktycznie niemożliwa, ale jeśli wystąpi,
        -- losujemy ponownie zamiast zwracać błąd zapisu zaświadczenia.
        EXIT WHEN NOT EXISTS (SELECT 1 FROM certificates WHERE verification_code = code);
    END LOOP;
    RETURN code;
END;
$$;

-- 2. Kolumna (na razie bez DEFAULT i NOT NULL, żeby uzupełnić istniejące wiersze) i indeks
--    unikalny. Indeks powstaje PRZED uzupełnieniem: sprawdzenie kolizji w generatorze
--    korzysta z niego, a bez indeksu każdy wiersz oznaczałby pełny skan tabeli
--    (na kopii bazy 72 s zamiast ~1 s). NULL-e nie naruszają unikalności.
ALTER TABLE certificates ADD COLUMN verification_code text;

CREATE UNIQUE INDEX certificates_verification_code_uidx
    ON certificates (verification_code);

-- 3. Uzupełnienie istniejących zaświadczeń.
UPDATE certificates
SET verification_code = generate_certificate_verification_code()
WHERE verification_code IS NULL;

-- 4. Reguły docelowe.
ALTER TABLE certificates
    ALTER COLUMN verification_code SET DEFAULT generate_certificate_verification_code(),
    ALTER COLUMN verification_code SET NOT NULL,
    ADD CONSTRAINT certificates_verification_code_format
        CHECK (verification_code ~ '^[2-9A-HJ-NP-Z]{12}$');

-- 5. Niezmienność kodu przez całe życie dokumentu.
CREATE FUNCTION certificates_verification_code_immutable() RETURNS trigger
    LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.verification_code IS DISTINCT FROM OLD.verification_code THEN
        RAISE EXCEPTION 'certificates.verification_code is immutable (certificate id %)', OLD.id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER certificates_verification_code_immutable
    BEFORE UPDATE OF verification_code ON certificates
    FOR EACH ROW
    EXECUTE FUNCTION certificates_verification_code_immutable();
