# Base Network Indexer

A basic indexer for the Base network that stores transaction data for a configured set of addresses in a PostgreSQL database and exposes it via a REST API.

## How to Install

1. **Clone the repository**

   ```sh
   git clone https://github.com/danilevy1212/baseidx-wt
   cd baseidx-wt
   ```

2. **Set up your environment**

   * If you're using `direnv` and Nix flakes:

     ```sh
     direnv allow .
     ```

   * Otherwise, make sure you have:

     * Go ≥ 1.24.3 installed
     * An environment variable loader such as `dotenv` to load `.env` variables

3. **Configure environment variables**

   ```sh
   cp .env.example .env
   ```

## How to Run

This project provides three commands:

### 1. `dbCreate`: Initialize the database schema

Start the PostgreSQL database:

```sh
docker compose up db
```

Then run:

```sh
go run ./cmd/dbCreate
```

### 2. `index`: Index transaction data

Starts the indexer, which scrapes the Base network and stores transactions in the database:

```sh
go run ./cmd/index
```

The individual blocks travelled and address list are configured via environment variables.

### 3. `api`: Start the REST API

```sh
go run ./cmd/api
```

This exposes the following endpoints:

* `GET /accounts/0x.../balance`
* `GET /accounts/0x.../transactions`
* `GET /transactions?start=...&end=...`

Example requests are available via the provided [Bruno](https://www.usebruno.com/) and Postman collections in the `devtools/` folder.
