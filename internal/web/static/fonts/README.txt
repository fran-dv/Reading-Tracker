Alegreya Sans (Regular, Medium) and Alegreya Italic, by Huerta Tipográfica,
under the SIL Open Font License 1.1 (OFL.txt).

Source: github.com/google/fonts, ofl/alegreyasans and ofl/alegreya.
Alegreya Italic is the wght=400 instance of the variable font.
All three are subset to Latin-1 plus common punctuation and saved as WOFF2:

  fonttools varLib.instancer Alegreya-Italic[wght].ttf wght=400 -o Alegreya-Italic.ttf
  pyftsubset <font>.ttf --flavor=woff2 \
    --unicodes="U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2190-2199,U+2212,U+2215,U+FEFF,U+FFFD" \
    --layout-features='kern,liga,calt,lnum,onum,pnum,tnum,smcp,c2sc,case,frac,ss01'
