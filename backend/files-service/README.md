# Files servis

Mikroservis za metapodatke o folderima i datotekama (Go + MongoDB).
Pokriva: **T-10, T-11, T-12, T-13, T-20** (folderi), otpremanje datoteke
**T-23** (sinhrona provera kvote kod Users servisa)
**T-24** (otpremanje nove verzije datoteke)
i **T-25** (azuriranje zauzeca skladista korisnika).

## Struktura

```
cmd/main.go                 pokretanje servisa, rute, graceful shutdown
internal/config             citanje konfiguracije iz env promenljivih (2.4)
internal/db                 konekcija na Mongo + indeksi            (T-11)
internal/model              entiteti Folder i File                  (T-11, 2.1)
internal/repository         upiti nad Mongo kolekcijama
internal/service            poslovna logika + validacija + vlasnistvo (T-12, T-13)
internal/handler            HTTP sloj, mapiranje gresaka na status kodove
internal/middleware         auth (X-User-ID), CORS, logovanje
```

## Pokretanje

Prvi put (generise `go.sum`):

```bash
cd backend/files-service
go mod tidy
```

Preko Docker Compose-a (iz korena projekta):

```bash
cp .env.example .env      # ako vec nije
docker compose up --build mongodb files-service
```

Lokalno, bez Docker-a (Mongo mora da radi):

```bash
MONGO_URI=mongodb://localhost:27017 go run ./cmd
```

Testovi poslovne logike (ne traze Mongo):

```bash
go test ./...
```

## Konfiguracija (2.4)

| Promenljiva | Podrazumevano | Opis |
|---|---|---|
| `FILES_SERVICE_PORT` | `8082` | port servisa |
| `MONGO_URI` | `mongodb://localhost:27017` | adresa Mongo baze |
| `FILES_DB_NAME` | `files_db` | ime baze |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:4200` | samo za lokalni rad bez gateway-a; u Docker-u CORS resava gateway |
| `DB_TIMEOUT` | `5s` | timeout jedne operacije nad bazom |
| `STORAGE_PATH` | `./data/files` | gde se cuva sadrzaj datoteka |
| `MAX_UPLOAD_SIZE` | `52428800` (50 MB) | najveci dozvoljeni upload, u bajtovima |
| `USERS_SERVICE_URL` | `http://localhost:8081` | adresa Users servisa (provera kvote) |
| `USERS_TIMEOUT` | `3s` | koliko se ceka odgovor Users servisa |
| `ALLOWED_FILE_TYPES` | prazno = lista u kodu | bela lista MIME tipova, zarezom odvojeni (2.18) |

## Endpointi

Svi zahtevi (osim `/health`) moraju imati header `X-User-ID`.
Privremeno resenje dok ne proradi prijava: API gateway ce kasnije iz JWT-a
izvuci ID korisnika i postaviti ovaj header.

| Metod | Putanja | Opis |
|---|---|---|
| `GET` | `/health` | provera da li servis radi |
| `POST` | `/folders` | kreiranje foldera (T-12) |
| `GET` | `/folders/root/contents` | sadrzaj korenskog foldera (T-20) |
| `GET` | `/folders/{id}/contents` | sadrzaj foldera + breadcrumbs (T-20) |
| `PATCH` | `/folders/{id}` | preimenovanje foldera (1.6) |
| `POST` | `/files` | otpremanje datoteke (1.5) |
| `PUT` | `/files/{id}` | otpremanje nove verzije datoteke (1.5, T-24) |
| `PATCH` | `/files/{id}` | preimenovanje datoteke (1.6) |
| `GET` | `/files/{id}/verify` | provera integriteta sadrzaja (2.18) |

### Primeri

```bash
# folder u korenu
curl -X POST localhost:8082/folders \
  -H 'X-User-ID: dev-user-1' -H 'Content-Type: application/json' \
  -d '{"name":"Fakultet"}'

# podfolder (parentId je ID vracen iz prethodnog poziva)
curl -X POST localhost:8082/folders \
  -H 'X-User-ID: dev-user-1' -H 'Content-Type: application/json' \
  -d '{"name":"SOA","parentId":"<ID>"}'

# sadrzaj korena
curl localhost:8082/folders/root/contents -H 'X-User-ID: dev-user-1'

# preimenovanje
curl -X PATCH localhost:8082/folders/<ID> \
  -H 'X-User-ID: dev-user-1' -H 'Content-Type: application/json' \
  -d '{"name":"SOA projekat"}'
```

### Otpremanje datoteke

`POST /files` prima `multipart/form-data` sa poljima:

| Polje | Obavezno | Opis |
|---|---|---|
| `file` | da | sama datoteka |
| `folderId` | ne | ID foldera; prazno ili izostavljeno znaci koren |

```bash
# u koren
curl -X POST localhost:8082/files \
  -H 'X-User-ID: dev-user-1' \
  -F 'file=@beleske.txt'

# u odredjeni folder
curl -X POST localhost:8082/files \
  -H 'X-User-ID: dev-user-1' \
  -F 'file=@slika.png' -F 'folderId=<ID foldera>'
```

Kako to radi iznutra:

1. ID dokumenta se generise unapred, jer od njega nastaje `storageKey`
2. sadrzaj se upisuje u storage (lokalni disk, kasnije MinIO)
3. metapodaci se upisuju u Mongo

Ako treci korak pukne, sadrzaj se brise iz storage-a - da ne ostanu bajtovi na
koje nijedan dokument ne pokazuje. Ako pukne drugi korak, u bazi nema nicega.

### Nova verzija datoteke (1.5, T-24)

`PUT /files/{id}` prima isti `multipart/form-data`, ali samo polje `file`.
Naziv, folder i ID ostaju isti - menja se sadrzaj, velicina, MIME tip,
broj verzije i vreme poslednje izmene.

```bash
curl -X PUT localhost:8082/files/<ID datoteke> \
  -H 'X-User-ID: dev-user-1' \
  -F 'file=@beleske.txt'
```

Istorija verzija se **ne cuva** - specifikacija kaze da se sadrzaj zamenjuje
i belezi vreme poslednje izmene, pa `version` sluzi samo kao brojac izmena.

Novi sadrzaj se upisuje pod **nov kljuc**, a stari se brise tek kada metapodaci
budu uspesno azurirani. Da se pise preko starog kljuca, prekid u tom trenutku
bi ostavio datoteku sa starim metapodacima i novim sadrzajem.

Kvota se proverava samo na **prirast** (`nova velicina - stara velicina`).
Manja ili jednaka verzija ne zauzima nista novo, pa se Users servis ni ne zove.

### Azuriranje zauzeca skladista (T-25)

Posle svakog uspesnog otpremanja Files servis javlja Users servisu koliko se
zauzece promenilo:

```
POST {USERS_SERVICE_URL}/users/{ownerId}/storage
{"delta": 1234}
```

`delta` je razlika u bajtovima i moze biti negativna (manja nova verzija, a
kasnije i brisanje datoteke). Prihvata se bilo koji 2xx odgovor.

> **Za Users servis:** ovaj endpoint jos ne postoji - treba dodati rukovalac
> koji na `POST /users/{id}/storage` uveca `used_storage` za `delta`, bez
> spustanja ispod nule. Dok ga nema, otpremanje i dalje radi, ali se u logu
> Files servisa pojavljuje upozorenje i `used_storage` ostaje nepromenjen.

Poziv se salje **tek posto je datoteka sacuvana** i njegov neuspeh **ne obara**
zahtev: korisniku bi inace bilo vraceno "greska" za nesto sto je proslo. Cena
je da zauzece moze privremeno da zaostane za stvarnim stanjem. Ispravno resenje
je ponavljanje poziva preko reda poruka (outbox), sto izlazi iz obima sprinta -
zato se neuspeh glasno loguje.

Zajedno sa T-23 ovo zatvara krug: provera kvote pre otpremanja ima smisla samo
ako neko i uvecava zauzece posle njega.

### Provera kvote (2.5, T-23)

Pre nego sto primi i jedan bajt, Files servis **sinhrono** pita Users servis
koliko je korisniku ostalo prostora:

```
GET {USERS_SERVICE_URL}/users/{ownerId}/quota
-> {"storage_quota": 1073741824, "used_storage": 0, "available_storage": 1073741824}
```

Ako datoteka ne staje, otpremanje se odbija sa `507`. Poziv ide **direktno**
ka Users servisu, a ne kroz API gateway: gateway trazi korisnikov JWT, a Files
servis ga nema - on govori u svoje ime, ne u ime korisnika.

Redosled provera je namerno ovakav: naziv i vlasnistvo nad folderom proveravaju
se lokalno i besplatno, pa tek onda ide mrezni poziv.

Files servis u pozivu salje `X-User-ID` vlasnika, jer Users servis dozvoljava
citanje kvote samo vlasniku ili adminu.

Ako Users servis ne odgovori, otpremanje se **odbija** (`503`), a ne propusta -
bolje odbiti nego dozvoliti probijanje kvote. Blaza varijanta, u kojoj servis
nastavlja da radi i kada susedni servis padne, je zahtev 2.6.

### Bela lista tipova (2.18)

Tip datoteke se odredjuje **po sadrzaju** - prvih 512 bajtova ("magicni
bajtovi") kroz `http.DetectContentType` - a ne po ekstenziji ili zaglavlju koje
salje klijent, jer oba moze da se podmetne. Ako tip nije na listi, odgovor je
`415`, a nista ne stize ni do skladista ni do baze.

Podrazumevano su dozvoljeni tekst, PDF, slike, ZIP (ukljucujuci .docx/.xlsx/
.pptx), MP3, WAV, MP4 i WebM. **Namerno nisu** dozvoljeni `text/html` i
`text/xml` (ukljucujuci SVG), jer mogu da nose JavaScript i posluze za XSS.
Izvrsni fajlovi se prepoznaju kao `application/octet-stream` i takodje padaju.

Klijent ima i svoju proveru (velicina, ocigledno opasne ekstenzije), ali samo
radi brze povratne informacije - obavezujuca je provera na serveru.

### Integritet datoteka (2.18)

Pri otpremanju se SHA-256 racuna "u prolazu", dok se sadrzaj upisuje, i cuva
u polju `checksum`. `GET /files/{id}/verify` ponovo cita sacuvani sadrzaj,
racuna otisak i poredi:

```json
{"fileId": "...", "expected": "ba78...", "actual": "ba78...", "valid": true}
```

Demonstracija: otpremi datoteku, pa izmeni njen fajl direktno u volume-u
`files_data` (mimo aplikacije) - provera ce vratiti `"valid": false`.

### Status kodovi

| Kod | Kada |
|---|---|
| `400` | neispravan JSON, nevalidan naziv ili nevalidan ID |
| `401` | nedostaje `X-User-ID` |
| `403` | folder pripada drugom korisniku (T-13) |
| `404` | folder ne postoji |
| `409` | folder ili datoteka sa istim nazivom vec postoji na istom mestu |
| `413` | datoteka je veca od `MAX_UPLOAD_SIZE` |
| `415` | tip sadrzaja nije na beloj listi |
| `503` | Users servis ne odgovara, kvota se ne moze proveriti |
| `507` | korisnik nema dovoljno slobodnog prostora |

## Model podataka (2.1)

**folders**: `_id`, `name`, `ownerId`, `parentId` (null = koren), `ancestors[]`,
`createdAt`, `updatedAt`

`ancestors` je niz ID-jeva svih nadredjenih foldera, od korena ka roditelju.
Zahvaljujuci njemu breadcrumbs se dobija jednim upitom, a kasnije ce olaksati
brisanje celog podstabla (1.13) i deljenje foldera sa celim sadrzajem (1.10).

**files**: `_id`, `name`, `ownerId`, `folderId`, `mimeType`, `size`,
`storageKey` (referenca na sadrzaj u object store-u), `version`,
`createdAt`, `updatedAt`

Indeksi: jedinstven `(ownerId, parentId, name)` za foldere i
`(ownerId, folderId, name)` za datoteke - baza sama sprecava duplikate naziva.

## Sta jos nije uradjeno (nije potrebno za ocenu 6)

- MinIO umesto lokalnog diska (2.7) - pise se nova implementacija interfejsa
  `storage.Storage` i menja samo `cmd/main.go`
- preuzimanje i pregled datoteke (1.9)
- brisanje (1.13), deljenje (1.10), pretraga (1.11)
