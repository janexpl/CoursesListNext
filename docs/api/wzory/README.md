# Wzory drukowanych dokumentów

## `blankiet-zaswiadczenia.pdf`

Skan blankietu zaświadczenia używanego przez organizatora (Konica Minolta, 200 dpi, dwie
strony: przód z monogramem i flagą UE, odwrót z samą ramką). To źródło wzoru giloszowego
wkompilowanego w API jako `api/internal/guilloche/pattern/{front,back}.png`.

Z tego skanu do wzoru drukowanego prowadzi taka droga:

1. wyciągnięcie obrazów stron (`pdfimages -j`),
2. maska atramentu z kanału czerwonego (zielony atrament pochłania czerwień),
   wyprostowanie (`-deskew`) i przycięcie do arkusza,
3. usunięcie flagi UE przez nałożenie fragmentu **tej samej** strony przesuniętego
   o dziesięć okresów siatki (okres to 27 px przy 200 dpi) - fragment z drugiej strony
   zostawiał widoczny szew, bo oba skany trzymają siatkę w innej fazie,
4. przemalowanie na jednolitą zieleń `#3FBF4F` i zapis jako PNG z ośmiokolorową paletą
   w rozdzielczości 1240 x 1754 px (150 dpi na A4).

Flaga UE jest z wzoru celowo usunięta - decyzja właściciela systemu.
