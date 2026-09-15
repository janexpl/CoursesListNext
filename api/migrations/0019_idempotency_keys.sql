-- Klucze idempotencji dla POST /certificates (nagłówek Idempotency-Key).
--
-- Platforma e-learningowa ponawia wystawienie zaświadczenia do 5 razy. Wiersz
-- zapisywany jest w tej samej transakcji co zaświadczenie, więc ponowienie po
-- zerwanym połączeniu albo timeoucie nie wystawi drugiego dokumentu:
--   - key            - wartość nagłówka; klucz główny jest ograniczeniem unikalności,
--   - request_hash   - sha256 znormalizowanego ciała żądania (ten sam klucz
--                      z innym ciałem kończy się 409),
--   - certificate_id - zaświadczenie wystawione pod tym kluczem,
--   - created_at     - wpisy starsze niż 30 dni API usuwa cyklicznie; po tym czasie
--                      ten sam klucz wystawiłby nowe zaświadczenie.
--
-- Wpływ na istniejące dane: brak. Migracja tylko dodaje pustą tabelę - zaświadczenia
-- wystawione wcześniej nie mają klucza i nie da się ich "odtworzyć" ponowieniem.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

CREATE TABLE idempotency_keys (
    key text PRIMARY KEY,
    request_hash text NOT NULL,
    certificate_id bigint NOT NULL REFERENCES certificates(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT idempotency_keys_key_length CHECK (char_length(key) BETWEEN 1 AND 255)
);

-- Sprzątanie wpisów starszych niż 30 dni.
CREATE INDEX idempotency_keys_created_at_idx ON idempotency_keys (created_at);
