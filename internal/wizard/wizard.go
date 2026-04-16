package wizard

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// ServerConfig representa la configuración del servidor
type ServerConfig struct {
	Domain        string
	AuthToken     string
	HTTPPort      string
	HTTPSEnabled  bool
	HTTPSPort     string
	ACMEEmail     string
	ControlPort   string
	MaxConnsPerIP string
}

// ClientConfig representa la configuración del cliente
type ClientConfig struct {
	Server    string
	AuthToken string
}

const logoRed = "\033[38;5;203m"
const logoReset = "\033[0m"
const logoBold = "\033[1m"
const logoGray = "\033[38;5;245m"

func printLogo() {
	logo := logoRed + `
                           ++++++                            
                        +++++++++++++                        
                      +++++++++++++++++                      
                     +++++++++++++++++++                     
                    ++++++++++++++++++++++++                 
             +++++++++++++++++++++++++++++++++               
           ++++++++++++++++++++++++++++++++++++++++          
          ++++++++++++++++++++++++++++++++++++++++++++       
         ++++++++++++++++++++++++++++++++++++++++++++++      
         ++++++++++++++++++++++++++++++++++++++++++++++      
         ++++++++++++++++++++++++++++++++++++++++++++++      
          ++++++++++++++++++++++++++++++++++++++++++++       
            ++++++++++++++++++++++++++++++++++++++++         
                                                             
            +++++++++++++++++++++++++++++++++                
                       +++++++++++                           
` + logoReset + `
`
	fmt.Print(logo)
}

// RunServerWizard ejecuta el wizard interactivo para configurar smuf-server
func RunServerWizard() (*ServerConfig, error) {
	printLogo()
	fmt.Println(logoBold + "  Configuración del servidor" + logoReset)
	fmt.Println(logoGray + "  Responde unas preguntas y estarás listo." + logoReset)
	fmt.Println()

	cfg := &ServerConfig{
		HTTPPort:      "8080",
		HTTPSPort:     "443",
		ControlPort:   "7000",
		MaxConnsPerIP: "5",
	}

	// Paso 1: Dominio
	fmt.Println(logoGray + "  [1/3] Dominio" + logoReset)
	if err := survey.AskOne(&survey.Input{
		Message: "Tu dominio (ej: miapp.com):",
		Help:    "Los túneles serán como abc123.tudominio.com\nUsa 'localhost' para pruebas locales",
		Default: "localhost",
	}, &cfg.Domain, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}
	fmt.Println()

	// Paso 2: Token de autenticación
	fmt.Println(logoGray + "  [2/3] Seguridad" + logoReset)

	// Siempre generar token automáticamente para simplificar
	token, err := generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("error generando token: %w", err)
	}
	cfg.AuthToken = token

	fmt.Println()
	fmt.Println(logoRed + "  ✓" + logoReset + " Token generado automáticamente:")
	fmt.Println()
	fmt.Println(logoBold + "    " + token + logoReset)
	fmt.Println()
	fmt.Println(logoGray + "    └─ Copia este token, lo necesitarás en tu ordenador" + logoReset)
	fmt.Println()

	// Paso 3: HTTPS
	fmt.Println(logoGray + "  [3/3] HTTPS" + logoReset)
	if cfg.Domain != "localhost" {
		if err := survey.AskOne(&survey.Confirm{
			Message: "¿Activar HTTPS automático?",
			Help:    "Certificado gratis con Let's Encrypt. Requiere puertos 80 y 443 abiertos.",
			Default: true,
		}, &cfg.HTTPSEnabled); err != nil {
			return nil, err
		}

		if cfg.HTTPSEnabled {
			cfg.HTTPPort = "80"
			if err := survey.AskOne(&survey.Input{
				Message: "Tu email (para avisos de certificado):",
				Help:    "Opcional. Let's Encrypt te avisará si el certificado va a caducar.",
			}, &cfg.ACMEEmail); err != nil {
				return nil, err
			}
		}
	} else {
		fmt.Println(logoGray + "  (HTTPS no disponible en localhost)" + logoReset)
	}
	fmt.Println()

	// Guardar automáticamente - es lo más sencillo
	if err := saveServerEnv(cfg); err != nil {
		fmt.Printf(logoRed+"  ⚠"+logoReset+" No se pudo guardar .env: %v\n", err)
	} else {
		fmt.Println(logoRed + "  ✓" + logoReset + " Configuración guardada en " + logoBold + ".env" + logoReset)
	}

	// Resumen final
	fmt.Println()
	fmt.Println(logoBold + "  ¡Listo! Tu servidor está configurado." + logoReset)
	fmt.Println()
	fmt.Println(logoGray + "  Próximos pasos:" + logoReset)
	fmt.Println("  1. En tu ordenador, ejecuta: " + logoBold + "smuf --setup" + logoReset)
	fmt.Println("  2. Pega el token cuando te lo pida")
	fmt.Println("  3. Luego solo: " + logoBold + "smuf 3000" + logoReset + " (o el puerto de tu app)")
	fmt.Println()

	return cfg, nil
}

// RunClientWizard ejecuta el wizard interactivo para configurar smuf
func RunClientWizard() (*ClientConfig, error) {
	printLogo()
	fmt.Println(logoBold + "  Conectar a tu servidor" + logoReset)
	fmt.Println(logoGray + "  Solo 2 datos y listo." + logoReset)
	fmt.Println()

	cfg := &ClientConfig{}

	// Paso 1: Servidor
	fmt.Println(logoGray + "  [1/2] Servidor" + logoReset)
	if err := survey.AskOne(&survey.Input{
		Message: "Dirección de tu servidor:",
		Help:    "El dominio o IP donde instalaste smuf-server\nEjemplo: miapp.com:7000 o 123.45.67.89:7000",
		Default: "localhost:7000",
	}, &cfg.Server, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}
	fmt.Println()

	// Paso 2: Token
	fmt.Println(logoGray + "  [2/2] Token" + logoReset)
	if err := survey.AskOne(&survey.Input{
		Message: "Pega el token del servidor:",
		Help:    "El token que se generó cuando configuraste smuf-server\nSi no tienes token, déjalo vacío",
	}, &cfg.AuthToken); err != nil {
		return nil, err
	}
	fmt.Println()

	// Guardar automáticamente
	if err := saveClientEnv(cfg); err != nil {
		fmt.Printf(logoRed+"  ⚠"+logoReset+" No se pudo guardar .env: %v\n", err)
	} else {
		fmt.Println(logoRed + "  ✓" + logoReset + " Configuración guardada en " + logoBold + ".env" + logoReset)
	}

	// Resumen final
	fmt.Println()
	fmt.Println(logoBold + "  ¡Listo!" + logoReset + " Ahora solo ejecuta:")
	fmt.Println()
	fmt.Println("    " + logoBold + "smuf 3000" + logoReset + logoGray + "   (o el puerto de tu app)" + logoReset)
	fmt.Println()

	return cfg, nil
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func saveServerEnv(cfg *ServerConfig) error {
	var lines []string
	lines = append(lines, "# smuf-server configuration")
	lines = append(lines, fmt.Sprintf("SMUF_DOMAIN=%s", cfg.Domain))
	if cfg.AuthToken != "" {
		lines = append(lines, fmt.Sprintf("SMUF_AUTH_TOKEN=%s", cfg.AuthToken))
	}
	lines = append(lines, fmt.Sprintf("SMUF_CONTROL_PORT=%s", cfg.ControlPort))
	lines = append(lines, fmt.Sprintf("SMUF_HTTP_PORT=%s", cfg.HTTPPort))
	if cfg.HTTPSEnabled {
		lines = append(lines, "SMUF_HTTPS=true")
		lines = append(lines, fmt.Sprintf("SMUF_HTTPS_PORT=%s", cfg.HTTPSPort))
		if cfg.ACMEEmail != "" {
			lines = append(lines, fmt.Sprintf("SMUF_ACME_EMAIL=%s", cfg.ACMEEmail))
		}
	}
	lines = append(lines, fmt.Sprintf("SMUF_MAX_CONNS_PER_IP=%s", cfg.MaxConnsPerIP))

	return os.WriteFile(".env", []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func saveClientEnv(cfg *ClientConfig) error {
	var lines []string
	lines = append(lines, "# smuf client configuration")
	lines = append(lines, fmt.Sprintf("SMUF_SERVER=%s", cfg.Server))
	if cfg.AuthToken != "" {
		lines = append(lines, fmt.Sprintf("SMUF_AUTH_TOKEN=%s", cfg.AuthToken))
	}

	return os.WriteFile(".env", []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

// LoadEnvFile carga variables de entorno desde un archivo .env si existe
func LoadEnvFile() {
	envPath := ".env"

	// También buscar junto al ejecutable
	if exe, err := os.Executable(); err == nil {
		altPath := filepath.Join(filepath.Dir(exe), ".env")
		if _, err := os.Stat(altPath); err == nil {
			envPath = altPath
		}
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		return // No hay .env, no pasa nada
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Solo setear si no está ya definida (env vars tienen prioridad)
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
}

// ShouldRunWizard determina si debemos mostrar el wizard interactivo
func ShouldRunWizard(args []string) bool {
	// Si hay argumentos -h, --help, etc., no wizard
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return false
		}
	}
	return true
}
