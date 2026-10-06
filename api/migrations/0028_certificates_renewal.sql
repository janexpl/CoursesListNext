-- Przedłużenie zaświadczenia: stary dokument wskazuje swojego następcę.
--
-- Gdy termin ważności dobiega końca, kursant przechodzi szkolenie okresowe i dostaje nowy
-- dokument. POST /certificates/{id}/renew wystawia go dla tego samego kursanta i kursu,
-- a stary dostaje znacznik "przedłużone".
--
-- To NIE jest unieważnienie: dokument zostaje ważny do swojej daty, dalej się drukuje
-- i dalej weryfikuje publicznie jako ważny. Znacznik wyłącza go WYŁĄCZNIE z przypomnień
-- o wygasaniu i z kafelka "Wygasające" na pulpicie.
--
--   - renewed_at                 - kiedy dokument został przedłużony,
--   - renewed_by_certificate_id  - następca (nowe zaświadczenie),
--   - renewed_by_user_id         - kto przedłużenie wykonał.
--
-- Kierunek: STARY wskazuje NOWY. Filtry zestawień wygasających pracują na wierszu starego
-- dokumentu, więc warunek musi być lokalnym predykatem (renewed_at IS NULL), bez joinu
-- i bez podzapytania - tak samo jak istniejące revoked_at IS NULL. Wskazanie odwrotne
-- ("czego przedłużeniem jest ten dokument") wyliczamy zapytaniem po indeksie: dwie kolumny
-- mogłyby się rozjechać, jedna nie może.
--
-- Jedna kolumna na wierszu z natury pilnuje, że dokument ma najwyżej jednego następcę.
-- Indeks unikalny pilnuje odwrotności: ten sam następca nie przedłuża dwóch dokumentów.
--
-- Bez kolumny na powód: przedłużenie ma zawsze ten sam powód - kończy się ważność.
-- Kto i kiedy zapisuje dziennik zmian.
--
-- ON DELETE RESTRICT, bo kasowanie zaświadczenia w aplikacji jest miękkie (deleted_at),
-- więc w normalnej pracy więz nie zadziała. Zadziała przy ręcznym DELETE w psql i tam ma
-- zadziałać: zamiast zostawić wiszące wskazanie, wymusza świadomą decyzję. Przy miękkim
-- skasowaniu następcy API czyści znacznik u poprzednika, żeby nie wypadł z przypomnień
-- dokument, którego zamiennik przestał istnieć.
--
-- Wpływ na istniejące dane: brak, nowe kolumny są NULL dla wszystkich zaświadczeń.
-- Historii NIE oznaczamy wstecz - przedłużenia liczą się od wdrożenia.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

ALTER TABLE certificates
    ADD COLUMN renewed_at timestamptz,
    ADD COLUMN renewed_by_certificate_id bigint REFERENCES certificates(id) ON DELETE RESTRICT,
    ADD COLUMN renewed_by_user_id bigint REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT certificates_renewed_consistency CHECK (
        (renewed_at IS NULL AND renewed_by_certificate_id IS NULL)
        OR (renewed_at IS NOT NULL AND renewed_by_certificate_id IS NOT NULL)
    ),
    ADD CONSTRAINT certificates_renewed_not_self CHECK (renewed_by_certificate_id <> id);

-- Miękko skasowany następca nie blokuje wystawienia kolejnego.
CREATE UNIQUE INDEX certificates_renewed_by_certificate_id_uidx
    ON certificates (renewed_by_certificate_id)
    WHERE renewed_by_certificate_id IS NOT NULL AND deleted_at IS NULL;
