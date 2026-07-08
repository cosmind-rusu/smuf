# Show HN: smuf — Self-hosted HN launch materials

## Title (lo que ves en la portada)

```
Show HN: smuf – self-hosted ngrok alternative written in Go
```

Alternativas por si la primera no encaja:

```
Show HN: smuf – open-source ngrok alternative you host yourself
```

```
Show HN: smuf – HTTP/TCP tunnel server you can deploy in seconds
```

## First comment (lo que pones como primer comentario)

```
Hey HN! I built smuf – a self-hosted alternative to ngrok. You deploy the
server on your own infrastructure (Railway, VPS, Raspberry Pi) and use a
lightweight CLI client to expose local ports.

Why another tunnel tool? I got tired of ngrok's paywall and rate limits for
something so simple. The self-hosted options out there (frp, bore, etc.)
either require complex configs or miss features I wanted. So I built smuf.

Features:
• HTTP + TCP tunnels (SSH, DBs, anything)
• Automatic HTTPS via Let's Encrypt
• Token authentication + per-IP rate limiting
• Custom subdomains (myapp.yourdomain.com)
• Multiple concurrent tunnels per process
• WebSockets support
• Real-time web dashboard (HashiCorp-style dark theme)
• One-click deploy on Railway
• Interactive setup wizard (no config file needed)
• Docker + pre-built binaries

Tech: written in Go, uses hashicorp/yamux for multiplexing. ~2,000 LOC,
zero external dependencies for the core. Apache 2.0 licensed.

Try it:
  https://github.com/cosmind-rusu/smuf

Server in 30 seconds:
  railway.com/deploy/high-warm  (or docker compose up -d)

Client:
  smuf 3000  # exposes localhost:3000

The project is early-stage but production-ready for personal/small-team use.
Would love feedback on the approach, the docs, or feature requests. What's
your current tunnel setup? Still paying ngrok?

Thanks for checking it out!
```

## Tips para la publicación

1. **Hora**: Publica entre las 14:00-17:00 UTC (hora de España: 16:00-19:00) para pillar a USA despertándose y Europa aún activa.

2. **No edites el título** después de publicar — HN lo penaliza. Mejor elige bien desde el principio.

3. **Responde a todos los comentarios** las primeras horas — HN premia la actividad. Sé humilde, agradece el feedback, no te pongas defensivo.

4. **Si no funciona al primer intento**, puedes reintentar con un título distinto días después. La gente hace "resubmit" semanas más tarde.

5. **Prepara el repo**: asegúrate de que el README esté impecable, que la gente que llegue desde HN entienda el proyecto en 10 segundos. El banner + los badges + los ejemplos de uso ya están bien, pero quizá añadir un GIF de demostración (asciinema o similar) ayuda mucho.
