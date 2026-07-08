<div align="center">
  <img src="banner.svg" width="100%" alt="smuf — self-hosted HTTP tunnel"/>
</div>

<br/>

<div align="center">
  <sub>SELF-HOSTED · OPEN SOURCE · WRITTEN IN GO</sub>
  <br/><br/>

  [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
  [![License: MIT](https://img.shields.io/badge/License-MIT-E53E3E?style=flat-square)](LICENSE)
  [![Self-hosted](https://img.shields.io/badge/self--hosted-✓-E53E3E?style=flat-square)]()
  [![yamux](https://img.shields.io/badge/yamux-multiplexing-656a76?style=flat-square)](https://github.com/hashicorp/yamux)
</div>

---

smuf expone un puerto local en una URL pública — como ngrok, pero **tuyo**: sin cuotas, sin límites, en tu propio servidor.

```
localhost:3000  ◄─────►  https://a3f1c9.tudominio.com
```

---

## Instalación

### Opción A: Descargar binario (recomendado)

Ve a [Releases](https://github.com/cdrusu/smuf/releases) y descarga el binario para tu sistema operativo.

| Binario | Dónde va |
|---|---|
| `smuf-server` | Tu VPS / servidor |
| `smuf` | Tu ordenador |

### Opción B: Docker

```bash
docker compose up -d
```

### Opción C: Compilar desde código

Necesitas [Go](https://go.dev/dl/) 1.25+.

```bash
git clone https://github.com/cdrusu/smuf.git && cd smuf
go build -o smuf-server ./cmd/smuf-server
go build -o smuf        ./cmd/smuf
```

---

## Uso

**Primera vez:** ejecuta sin argumentos y el wizard te configura todo:

```bash
./smuf-server   # en el servidor
./smuf --setup  # en tu ordenador
```

**Después:**

```bash
./smuf 3000                    # expone localhost:3000
./smuf 3000 4000 5000          # múltiples puertos a la vez
./smuf --sub miapp 3000        # URL fija: miapp.tudominio.com
./smuf --tcp 22                # túnel TCP puro (SSH, DB, etc.)
```

Resultado:

```
  Tunnel ready!

  Local   → http://localhost:3000
  Public  → https://a3f1c9.tudominio.com

  Press Ctrl+C to stop
```

---

## Dashboard

Mientras el servidor está corriendo, abre en el navegador:

```
http://tudominio.com:8080/
```

Diseñado con el estilo visual de [HashiCorp](https://www.hashicorp.com/): fondo oscuro `#0d0e12`, tipografía system-ui, tarjetas con micro-shadows y colores de acento azul (`#1060ff`).

Muestra todos los túneles activos con:
- Tipo de túnel (HTTP / TCP)
- URL pública
- Puerto local y dirección IP del cliente
- Tiempo activo

Se actualiza cada 5 s. El endpoint JSON está en `/_smuf/tunnels`.

---

## Configuración

Todo por variables de entorno (o archivo `.env` junto al ejecutable).

### Servidor (`smuf-server`)

| Variable | Default | Descripción |
|---|---|---|
| `SMUF_DOMAIN` | `localhost` | Tu dominio base |
| `SMUF_AUTH_TOKEN` | — | Token secreto (**obligatorio si expones el servidor a internet**: sin él, cualquiera puede crear túneles) |
| `SMUF_CONTROL_PORT` | `7000` | Puerto donde los clientes se conectan |
| `SMUF_HTTP_PORT` | `8080` | Puerto HTTP público |
| `SMUF_HTTPS` | `false` | HTTPS automático con Let's Encrypt |
| `SMUF_HTTPS_PORT` | `443` | Puerto HTTPS |
| `SMUF_ACME_EMAIL` | — | Email para avisos de certificado |
| `SMUF_MAX_CONNS_PER_IP` | `5` | Límite de túneles por IP |
| `SMUF_HANDSHAKE_TIMEOUT` | `10s` | Timeout del handshake |
| `SMUF_TCP_PORT_RANGE` | — | Rango de puertos TCP públicos (ej: `20000-30000`) |

### Cliente (`smuf`)

| Variable | Default | Descripción |
|---|---|---|
| `SMUF_SERVER` | `localhost:7000` | Dirección del servidor |
| `SMUF_AUTH_TOKEN` | — | Token (debe coincidir con el servidor) |
| `SMUF_SUBDOMAIN` | — | Subdominio fijo (equivalente a `--sub`) |

**Ejemplo `.env` servidor:**
```env
SMUF_DOMAIN=tudominio.com
SMUF_AUTH_TOKEN=un-token-secreto-largo
# SMUF_HTTPS=true
# SMUF_ACME_EMAIL=tu@email.com
```

**Ejemplo `.env` cliente:**
```env
SMUF_SERVER=tudominio.com:7000
SMUF_AUTH_TOKEN=un-token-secreto-largo
# SMUF_SUBDOMAIN=miapp
```

> Genera un token seguro con `openssl rand -hex 32`

---

## Cómo funciona

```
smuf 3000  ──TCP──►  smuf-server :7000
                            │
               "PORT 3000 SUB miapp"  →  "OK miapp https://miapp.tudominio.com"
                            │
                    yamux (multiplexación)
                            │
           petición → miapp.tudominio.com → stream yamux → localhost:3000
```

Usa [`hashicorp/yamux`](https://github.com/hashicorp/yamux) para multiplexar múltiples peticiones HTTP sobre una sola conexión TCP.

---

## Roadmap

- [x] HTTPS automático con Let's Encrypt
- [x] Autenticación por token
- [x] Rate limiting por IP
- [x] Dashboard web en tiempo real (estilo HashiCorp)
- [x] Múltiples túneles por proceso
- [x] Subdominio personalizado
- [x] WebSockets
- [x] Túneles TCP (no solo HTTP)
- [x] Imagen Docker oficial
- [x] Binarios pre-compilados
- [ ] Server-Sent Events (SSE)

---

## Estructura

```
smuf/
├── cmd/
│   ├── smuf/           # Cliente (tu ordenador)
│   └── smuf-server/    # Servidor (tu VPS)
└── internal/
    ├── tunnel/         # Registry + BufConn
    ├── wizard/         # Setup interactivo
    └── logger/         # Logging con timestamp
```

---

## Contribuir

¿Bug o idea? Abre un [issue](../../issues) o manda un pull request. Mira [CONTRIBUTING.md](CONTRIBUTING.md) para el flujo de trabajo.

---

<div align="center">
  <img src="logo.svg" width="40" alt="smuf"/>
  <br/>
  <sub>Hecho con Go · <a href="LICENSE">MIT License</a></sub>
</div>
