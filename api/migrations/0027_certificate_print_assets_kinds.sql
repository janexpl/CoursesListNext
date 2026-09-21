-- Trzy pieczątki zamiast dwóch, nazwane rolą zamiast numerem.
--
-- Organizator używa trzech różnych pieczątek: okrągłej, firmowej i imiennej. Numery
-- (pieczatka_1, pieczatka_2) nic autorowi szablonu nie mówiły - przy wstawianiu znacznika
-- trzeba było pamiętać, który numer to która pieczątka. Nazwy rodzajów są zarazem nazwami
-- znaczników szablonu, więc zmiana dotyczy obu stron naraz.
--
-- Każdy nadruk pozostaje opcjonalny: brak wgranego pliku oznacza, że znacznik znika
-- z wydruku bez śladu.
--
-- Wpływ na istniejące dane: na produkcji tabela jest pusta (migracja 0026 nie była tam
-- uruchamiana), więc UPDATE nie ma czego ruszać. W bazach deweloperskich dotychczasowe
-- rodzaje są mapowane po kolejności: pieczatka_1 -> okrągła, pieczatka_2 -> firmowa.
-- Uruchamiać przez: psql --single-transaction -v ON_ERROR_STOP=1 -f

ALTER TABLE certificate_print_assets
    DROP CONSTRAINT IF EXISTS certificate_print_assets_kind_check;

UPDATE certificate_print_assets SET kind = 'pieczatka_okragla' WHERE kind = 'pieczatka_1';
UPDATE certificate_print_assets SET kind = 'pieczatka_firmowa' WHERE kind = 'pieczatka_2';

ALTER TABLE certificate_print_assets
    ADD CONSTRAINT certificate_print_assets_kind_check
        CHECK (kind IN ('pieczatka_okragla', 'pieczatka_firmowa', 'pieczatka_imienna', 'podpis'));
