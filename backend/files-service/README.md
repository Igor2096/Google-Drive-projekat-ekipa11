# Files servis

Mikroservis za metapodatke o folderima i datotekama (Go + MongoDB).
Pokriva deo prvog sprinta: **T-10, T-11, T-12, T-13, T-20**.

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
| `CORS_ALLOWED_ORIGINS` | `http://localhost:4200` | dozvoljeni origin-i (zarezom odvojeni) |
| `DB_TIMEOUT` | `5s` | timeout jedne operacije nad bazom |

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

### Status kodovi

| Kod | Kada |
|---|---|
| `400` | neispravan JSON, nevalidan naziv ili nevalidan ID |
| `401` | nedostaje `X-User-ID` |
| `403` | folder pripada drugom korisniku (T-13) |
| `404` | folder ne postoji |
| `409` | folder sa istim nazivom vec postoji u istom roditelju |

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

## Sta jos nije uradjeno

- otpremanje datoteka i novih verzija (1.5) - `FileRepository` ima samo citanje
- sinhrona provera kvote kod Users servisa pri otpremanju (2.5)
- object store za sadrzaj datoteka (2.7) - zato postoji `storageKey`
- brisanje (1.13), deljenje (1.10), pretraga (1.11)
