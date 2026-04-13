<div align="center">
  <img src="banner.svg" width="100%" alt="smuf — self-hosted HTTP tunnel"/>
</div>

<br/>

<div align="center">

  <!-- Uppercase label style — Design.md §3 -->
  <sub>SELF-HOSTED · OPEN SOURCE · WRITTEN IN GO</sub>

  <br/><br/>

  [![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
  [![License: MIT](https://img.shields.io/badge/License-MIT-E53E3E?style=flat-square)](LICENSE)
  [![Self-hosted](https://img.shields.io/badge/self--hosted-✓-E53E3E?style=flat-square)]()
  [![yamux](https://img.shields.io/badge/yamux-multiplexing-656a76?style=flat-square)](https://github.com/hashicorp/yamux)

</div>

---

## ¿Qué es smuf?

¿Alguna vez quisiste mostrarle tu app a alguien sin tener que subirla a ningún servidor?

**smuf** resuelve exactamente eso. Con un solo comando, tu puerto local obtiene una URL pública que cualquiera puede visitar — como ngrok, pero **tuyo**: lo instalas en tu propio servidor, sin cuotas, sin límites y sin datos que salgan de tus manos.

```
Tu computadora          Tu servidor smuf
──────────────          ────────────────
localhost:3000  ◄─────►  http://a3f1c9.tudominio.com
```

---

## Instalación

Necesitas [Go](https://go.dev/dl/) 1.21 o superior.

```bash
git clone https://github.com/cdrusu/smuf.git
cd smuf

go build -o smuf-server ./cmd/smuf-server
go build -o smuf        ./cmd/smuf
```

Eso genera dos ejecutables:

| Archivo | Dónde va |
|---|---|
| `smuf-server` | Tu **servidor** (VPS, nube…) |
| `smuf` | Tu **computadora local** |

---

## Uso rápido

### PASO 1 — Levanta el servidor (una sola vez)

```bash
./smuf-server
```

```
[2026-04-13 12:00:00] INFO  smuf-server ready | control :7000 | http :8080 | domain localhost
[2026-04-13 12:00:00] INFO  http proxy listening on :8080
```

### PASO 2 — Abre un túnel desde tu máquina

Con tu app corriendo en `localhost:3000`:

```bash
./smuf 3000
```

```
  Tunnel ready!

  Local   → http://localhost:3000
  Public  → http://a3f1c9.localhost:8080

  Press Ctrl+C to stop
```

Comparte la URL pública. Cualquiera puede visitarla mientras el túnel esté abierto.

---

## Configuración

Sin archivos de config. Todo por variables de entorno:

### Servidor (`smuf-server`)

| Variable | Default | Descripción |
|---|---|---|
| `SMUF_CONTROL_PORT` | `7000` | Puerto donde los clientes se conectan |
| `SMUF_HTTP_PORT` | `8080` | Puerto HTTP público |
| `SMUF_DOMAIN` | `localhost` | Tu dominio base (`tudominio.com`) |

```bash
SMUF_DOMAIN=tudominio.com SMUF_HTTP_PORT=80 ./smuf-server
```

### Cliente (`smuf`)

| Variable | Default | Descripción |
|---|---|---|
| `SMUF_SERVER` | `localhost:7000` | Dirección de tu servidor |
| `SMUF_HTTP_PORT` | `8080` | Puerto HTTP del servidor |

```bash
SMUF_SERVER=tudominio.com:7000 ./smuf 3000
```

---

## Cómo funciona por dentro

```
[smuf 3000]  ──TCP──►  [smuf-server :7000]
                                │
              handshake:  "PORT 3000"  →  "OK a3f1c9"
                                │
              ▲ conexión elevada a yamux (multiplexación) ▲
                                │
   petición llega a  a3f1c9.tudominio.com:8080
                                │
              servidor abre yamux stream → cliente
                                │
   cliente reenvía la petición a localhost:3000
                                │
              respuesta viaja de vuelta por el stream
                                │
   respuesta entregada al navegador  ✓
```

**Tecnologías clave:**

| Paquete | Rol |
|---|---|
| [`hashicorp/yamux`](https://github.com/hashicorp/yamux) | Múltiples peticiones en paralelo sobre una sola conexión TCP |

---

## Roadmap

Funcionalidades planeadas — contribuciones bienvenidas:

- [ ] **HTTPS** — TLS automático con Let's Encrypt
- [ ] **Autenticación por token** — control de quién puede abrir túneles
- [ ] **Subdominio personalizado** — `miapp.tudominio.com` en lugar de un ID aleatorio
- [ ] **WebSockets** — soporte para apps en tiempo real
- [ ] **Dashboard web** — ver túneles activos desde el navegador
- [ ] **Múltiples túneles por cliente** — un proceso, varios puertos
- [ ] **Rate limiting** — protege tu servidor de abuso
- [ ] **Túneles TCP** — no solo HTTP, cualquier protocolo
- [ ] **Imagen Docker oficial** — despliegue en un comando
- [ ] **Binarios pre-compilados** — sin necesidad de tener Go instalado

---

## Estructura del proyecto

```
smuf/
├── cmd/
│   ├── smuf/           # Binario cliente (tu máquina)
│   └── smuf-server/    # Binario servidor (tu VPS)
└── internal/
    ├── tunnel/
    │   ├── conn.go     # BufConn — preserva bytes del handshake al pasar a yamux
    │   └── registry.go # Registry — mapa concurrente de sesiones activas
    └── logger/
        └── logger.go   # Info / Error / Fatal con timestamp
```

---

## Contribuir

¿Encontraste un bug? ¿Tienes una idea? Abre un [issue](../../issues) o manda un pull request.
Cualquier contribución es bienvenida.

---

<div align="center">
  <img src="logo.svg" width="40" alt="smuf cloud"/>
  <br/>
  <sub>Hecho con Go · <a href="LICENSE">MIT License</a></sub>
</div>
