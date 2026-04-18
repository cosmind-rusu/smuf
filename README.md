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

## Dashboard

Cuando el servidor está corriendo, abre en el navegador:

```
http://tudominio.com:8080/
```

Verás todos los túneles activos con su URL pública, puerto local, IP del cliente y tiempo activo. Se actualiza cada 5 segundos.

---

## Uso rápido

### Opción A: Wizard interactivo (recomendado para empezar)

Simplemente ejecuta sin argumentos y el wizard te guía:

```bash
# En tu servidor
./smuf-server
```

```
  🚀 Bienvenido a smuf-server
  ─────────────────────────────
  Vamos a configurar tu servidor en unos simples pasos.

? ¿Cuál es tu dominio? tudominio.com
? ¿Generar un token de autenticación automáticamente? Sí
  ✓ Token generado: a1b2c3d4...
? ¿Activar HTTPS automático con Let's Encrypt? Sí
? ¿Guardar configuración en archivo .env? Sí
  ✓ Configuración guardada en .env
```

En tu ordenador, lo mismo:

```bash
./smuf --setup
```

```
  🚀 Bienvenido a smuf
  ─────────────────────
  Vamos a conectarte a tu servidor.

? ¿Dirección de tu servidor smuf? tudominio.com:7000
? Token de autenticación: a1b2c3d4...
? ¿Guardar configuración en archivo .env? Sí
```

Después de configurar, solo necesitas:

```bash
./smuf 3000

# O varios puertos a la vez
./smuf 3000 4000 5000
```

### Opción B: Comando directo con variables de entorno

Si prefieres un solo comando:

```bash
# Servidor
SMUF_DOMAIN=tudominio.com SMUF_AUTH_TOKEN=mi-token ./smuf-server

# Cliente
SMUF_SERVER=tudominio.com:7000 SMUF_AUTH_TOKEN=mi-token ./smuf 3000
```

### Resultado

```
  Tunnel ready!

  Local   → http://localhost:3000
  Public  → https://a3f1c9b2d4e6f8a1.tudominio.com

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
| `SMUF_HTTPS` | `false` | Activa HTTPS automático con Let's Encrypt |
| `SMUF_HTTPS_PORT` | `443` | Puerto HTTPS público cuando `SMUF_HTTPS=true` |
| `SMUF_DOMAIN` | `localhost` | Tu dominio base (`tudominio.com`) |
| `SMUF_ACME_EMAIL` | *(vacío)* | Email opcional para avisos de Let's Encrypt |
| `SMUF_ACME_CACHE` | `certs` | Carpeta donde se guardan los certificados ACME |
| `SMUF_PUBLIC_HTTP_PORT` | *(vacío)* | Puerto HTTP que se anuncia si difiere del puerto local |
| `SMUF_PUBLIC_HTTPS_PORT` | *(vacío)* | Puerto HTTPS que se anuncia si difiere del puerto local |
| `SMUF_AUTH_TOKEN` | *(vacío)* | Token secreto para autenticar clientes (recomendado en producción) |
| `SMUF_MAX_CONNS_PER_IP` | `5` | Máximo de túneles simultáneos por IP |
| `SMUF_HANDSHAKE_TIMEOUT` | `10s` | Tiempo límite para completar el handshake |

También puedes forzar el wizard con `./smuf-server --setup`.

### Cliente (`smuf`)

| Variable | Default | Descripción |
|---|---|---|
| `SMUF_SERVER` | `localhost:7000` | Dirección de tu servidor |
| `SMUF_HTTP_PORT` | `8080` | Puerto HTTP del servidor cuando se conecta a un servidor antiguo que no devuelve URL pública |
| `SMUF_AUTH_TOKEN` | *(vacío)* | Token de autenticación (debe coincidir con el del servidor) |

```bash
SMUF_SERVER=tudominio.com:7000 ./smuf 3000
```

Si el servidor requiere autenticación:

```bash
SMUF_SERVER=tudominio.com:7000 SMUF_AUTH_TOKEN=mi-token-secreto ./smuf 3000
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

- [x] **HTTPS** — TLS automático con Let's Encrypt
- [x] **Autenticación por token** — control de quién puede abrir túneles
- [x] **Rate limiting** — protege tu servidor de abuso (límite por IP)
- [ ] **Subdominio personalizado** — `miapp.tudominio.com` en lugar de un ID aleatorio
- [ ] **WebSockets** — soporte para apps en tiempo real
- [x] **Dashboard web** — ver túneles activos desde el navegador
- [x] **Múltiples túneles por cliente** — un proceso, varios puertos
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

## Seguridad

### Archivo .env (recomendado)

La forma más fácil de configurar smuf es con un archivo `.env`. Crea el archivo junto al ejecutable:

**Servidor** (`.env` en tu VPS):
```env
# Obligatorio en producción
SMUF_DOMAIN=tudominio.com
SMUF_AUTH_TOKEN=pon-aqui-un-token-secreto-largo

# Opcional - descomentar para HTTPS automático
# SMUF_HTTPS=true
# SMUF_HTTP_PORT=80
# SMUF_HTTPS_PORT=443
# SMUF_ACME_EMAIL=tu@email.com
```

**Cliente** (`.env` en tu ordenador):
```env
SMUF_SERVER=tudominio.com:7000
SMUF_AUTH_TOKEN=pon-aqui-el-mismo-token-del-servidor
```

Luego ejecuta normalmente:
```bash
# En el servidor
./smuf-server

# En tu ordenador
./smuf 3000
```

> 💡 **Tip:** Genera un token seguro con `openssl rand -hex 32`

### Autenticación

Sin token configurado, cualquier persona que conozca la dirección de tu servidor podría crear túneles. **Siempre usa `SMUF_AUTH_TOKEN` en producción.**

### Rate Limiting

El servidor limita el número de túneles simultáneos por IP (default: 5). Configurable con `SMUF_MAX_CONNS_PER_IP`.

### Timeouts

- **Handshake:** 10 segundos (configurable con `SMUF_HANDSHAKE_TIMEOUT`)
- **HTTP Read:** 30 segundos
- **HTTP Write:** 60 segundos
- **HTTP Idle:** 120 segundos

### IDs de Túnel

Los IDs usan 128 bits de entropía (32 caracteres hex), generados con `crypto/rand`. Son prácticamente imposibles de adivinar.

### Recomendaciones

1. **Siempre usa `SMUF_AUTH_TOKEN`** en producción
2. **Activa HTTPS** con `SMUF_HTTPS=true` para cifrar el tráfico público
3. **Usa un firewall** para restringir el puerto de control (7000) solo a IPs conocidas si es posible
4. **Monitorea los logs** para detectar actividad sospechosa

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
