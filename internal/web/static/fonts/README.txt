Alegreya Sans (Regular, Medium), Alegreya Italic and Alegreya SC, by Huerta
Tipográfica, under the SIL Open Font License 1.1 (OFL.txt).

Source: github.com/google/fonts, ofl/alegreyasans, ofl/alegreya and
ofl/alegreyasc.
Alegreya Italic is the wght=400 instance of the variable font.
The first three are subset to Latin-1 plus common punctuation and saved as WOFF2:

  fonttools varLib.instancer Alegreya-Italic[wght].ttf wght=400 -o Alegreya-Italic.ttf
  pyftsubset <font>.ttf --flavor=woff2 \
    --unicodes="U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2190-2199,U+2212,U+2215,U+FEFF,U+FFFD" \
    --layout-features='kern,liga,calt,lnum,onum,pnum,tnum,smcp,c2sc,case,frac,ss01'

Alegreya SC Regular sets the generated book cover. It adds Latin Extended-A
(U+0100-017F) to the same set, for names such as Stanisław Lem or Karel Čapek:

  pyftsubset AlegreyaSC-Regular.ttf --flavor=woff2 \
    --unicodes="U+0000-00FF,U+0100-017F,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2190-2199,U+2212,U+2215,U+FEFF,U+FFFD" \
    --layout-features='kern,liga,calt,lnum,onum,pnum,tnum,smcp,c2sc,case,frac,ss01'

Glyph check (2026-11-04): in Alegreya SC the lower-case code points are
themselves true small capitals (height 505 against 649 for the capitals, at
1000 units per em), so no smcp feature is needed. Every code point of Basic
Latin, Latin-1 and Latin Extended-A is present except U+0149 (deprecated)
and U+017F (long s). Subset size: 31,784 bytes.
