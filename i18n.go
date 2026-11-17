package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type locale struct {
	code string
	tag  string
	name string
	// thousands separator, and the fewest digits a number needs before it is grouped
	group    string
	minGroup int
	msgs     map[string]string
}

// The first locale is the fallback.
var locales = []*locale{
	{code: "en", tag: "en-US", name: "English", group: ",", minGroup: 4, msgs: messagesEN},
	{code: "es", tag: "es-ES", name: "Español", group: ".", minGroup: 5, msgs: messagesES},
	{code: "de", tag: "de-DE", name: "Deutsch", group: ".", minGroup: 4, msgs: messagesDE},
	{code: "pl", tag: "pl-PL", name: "Polski", group: " ", minGroup: 5, msgs: messagesPL},
	{code: "fr", tag: "fr-FR", name: "Français", group: " ", minGroup: 4, msgs: messagesFR},
}

func findLocale(tag string) *locale {
	primary, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	primary, _, _ = strings.Cut(primary, "_")
	for _, l := range locales {
		if l.code == primary {
			return l
		}
	}
	return nil
}

func negotiate(acceptLanguage string) *locale {
	best, bestQ := locales[0], 0.0
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag, params, _ := strings.Cut(part, ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if q, _ = strconv.ParseFloat(v, 64); q > 1 {
				q = 0
			}
		}
		if l := findLocale(tag); l != nil && q > bestQ {
			best, bestQ = l, q
		}
	}
	return best
}

// localeFor takes ?lang= first, which the page's language links set, then Accept-Language.
func localeFor(r *http.Request) *locale {
	if l := findLocale(r.URL.Query().Get("lang")); l != nil {
		return l
	}
	return negotiate(r.Header.Get("Accept-Language"))
}

// t fills {name} placeholders from name, value pairs.
func (l *locale) t(key string, args ...any) string {
	msg, ok := l.msgs[key]
	if !ok {
		msg = messagesEN[key]
	}
	pairs := make([]string, 0, len(args))
	for i := 0; i+1 < len(args); i += 2 {
		pairs = append(pairs, "{"+fmt.Sprint(args[i])+"}", fmt.Sprint(args[i+1]))
	}
	return strings.NewReplacer(pairs...).Replace(msg)
}

// tn picks key+"One" for a count of 1 and fills {n}.
func (l *locale) tn(key string, n int64, args ...any) string {
	if n == 1 {
		key += "One"
	}
	return l.t(key, append([]any{"n", l.number(n)}, args...)...)
}

func (l *locale) number(n int64) string {
	d := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, d = "-", d[1:]
	}
	if len(d) < l.minGroup {
		return sign + d
	}
	for i := len(d) - 3; i > 0; i -= 3 {
		d = d[:i] + l.group + d[i:]
	}
	return sign + d
}

var messagesEN = map[string]string{
	"title":          "Write access",
	"titleNamed":     "{name}: write access",
	"nostrRelay":     "Nostr relay",
	"open":           "Anyone can publish to this relay right now, so there is nothing to pay for.",
	"notTakingRetry": "This relay is not taking payments right now. Try again later.",
	"sats":           "{n} sats",
	"forDays":        "for {n} days of write access",
	"forDaysOne":     "for {n} day of write access",
	"forPermanent":   "for permanent write access",
	"readingFree":    "Reading is free. Paying lets the public key below publish to this relay.",
	"yourKey":        "Your public key",
	"keyPlaceholder": "Paste your npub1…",
	"useExtension":   "Use extension",
	"forKey":         "For",
	"lightning":      "Lightning",
	"onchain":        "On-chain",
	"lnInvoice":      "Lightning invoice",
	"btcAddress":     "Bitcoin address",
	"openWallet":     "Open in wallet",
	"copyInvoice":    "Copy invoice",
	"copyAddress":    "Copy address",
	"copy":           "Copy",
	"copied":         "Copied",
	"addRelay":       "Add this relay to your Nostr client to publish here:",
	"addRelayNoURL":  "Add this relay to your Nostr client to publish here.",
	"relayAddress":   "Relay address",
	"newInvoice":     "Get a new invoice",
	"back":           "Back",
	"done":           "Done",
	"noscript":       "This page needs JavaScript to create an invoice.",
	"terms":          "Terms",
	"privacy":        "Privacy",
	"contact":        "Contact:",
	"language":       "Language",

	"badKey":         "Enter an npub (npub1…) or a 64-character hex public key.",
	"error":          "Something went wrong. Try again.",
	"checking":       "Checking…",
	"blocked":        "This public key may not publish to this relay.",
	"permanent":      "This public key already has permanent write access.",
	"activeUntil":    "This public key can publish until {date}.",
	"payAddDays":     "Pay {sats} to add {n} days",
	"payAddDaysOne":  "Pay {sats} to add {n} day",
	"payPermanent":   "Pay {sats} for permanent access",
	"pay":            "Pay {sats}",
	"memberExpired":  "This public key's write access has expired.",
	"notMember":      "This public key isn't signed up yet.",
	"creating":       "Creating invoice…",
	"gettingAddress": "Getting an on-chain address…",
	"waiting":        "Waiting for payment of {sats}. This invoice expires in {time}.",
	"pendingExpired": "This invoice has expired. If you paid it, wait here while the payment is checked; otherwise get a new invoice.",
	"processing":     "Payment received and waiting to confirm. You can close this page; access starts once it confirms.",
	"paidPermanent":  "Paid. This public key can now publish to the relay.",
	"paidUntil":      "Paid. This public key can publish to the relay until {date}.",
	"expired":        "This invoice expired without a payment.",
	"unreachable":    "The relay is not answering right now; still trying.",
	"noExtensionKey": "The extension did not share a public key.",

	"invalidKey":     "That is not a valid npub or hex public key.",
	"notTaking":      "This relay is not taking payments right now.",
	"badRequest":     "Bad request.",
	"tooManyFromYou": "Too many invoices from your address. Try again in a few minutes.",
	"tooManyWaiting": "Too many sign-ups are waiting for payment. Try again in a few minutes.",
	"paymentsDown":   "Payments are unavailable right now. Try again later.",
	"unknownInvoice": "Unknown invoice.",
	"noOnchain":      "This invoice can't be paid on-chain.",
	"onchainDown":    "On-chain payment is unavailable right now. Pay with Lightning, or try again later.",
	"memo":           "Write access to {relay} for {key}",
	"memoNoName":     "Relay write access for {key}",
}

var messagesES = map[string]string{
	"title":          "Acceso de escritura",
	"titleNamed":     "{name}: acceso de escritura",
	"nostrRelay":     "Relay de Nostr",
	"open":           "Ahora mismo cualquiera puede publicar en este relay, así que no hay nada que pagar.",
	"notTakingRetry": "Este relay no acepta pagos en este momento. Inténtalo de nuevo más tarde.",
	"sats":           "{n} sats",
	"forDays":        "por {n} días de acceso de escritura",
	"forDaysOne":     "por {n} día de acceso de escritura",
	"forPermanent":   "por acceso de escritura permanente",
	"readingFree":    "Leer es gratis. Al pagar, la clave pública de abajo puede publicar en este relay.",
	"yourKey":        "Tu clave pública",
	"keyPlaceholder": "Pega tu npub1…",
	"useExtension":   "Usar extensión",
	"forKey":         "Para",
	"lightning":      "Lightning",
	"onchain":        "On-chain",
	"lnInvoice":      "Factura Lightning",
	"btcAddress":     "Dirección de Bitcoin",
	"openWallet":     "Abrir en el monedero",
	"copyInvoice":    "Copiar factura",
	"copyAddress":    "Copiar dirección",
	"copy":           "Copiar",
	"copied":         "Copiado",
	"addRelay":       "Añade este relay a tu cliente de Nostr para publicar aquí:",
	"addRelayNoURL":  "Añade este relay a tu cliente de Nostr para publicar aquí.",
	"relayAddress":   "Dirección del relay",
	"newInvoice":     "Obtener una factura nueva",
	"back":           "Volver",
	"done":           "Listo",
	"noscript":       "Esta página necesita JavaScript para crear una factura.",
	"terms":          "Términos",
	"privacy":        "Privacidad",
	"contact":        "Contacto:",
	"language":       "Idioma",

	"badKey":         "Introduce una npub (npub1…) o una clave pública hexadecimal de 64 caracteres.",
	"error":          "Algo salió mal. Inténtalo de nuevo.",
	"checking":       "Comprobando…",
	"blocked":        "Esta clave pública no puede publicar en este relay.",
	"permanent":      "Esta clave pública ya tiene acceso de escritura permanente.",
	"activeUntil":    "Esta clave pública puede publicar hasta el {date}.",
	"payAddDays":     "Pagar {sats} para añadir {n} días",
	"payAddDaysOne":  "Pagar {sats} para añadir {n} día",
	"payPermanent":   "Pagar {sats} por acceso permanente",
	"pay":            "Pagar {sats}",
	"memberExpired":  "El acceso de escritura de esta clave pública ha caducado.",
	"notMember":      "Esta clave pública aún no está registrada.",
	"creating":       "Creando factura…",
	"gettingAddress": "Obteniendo una dirección on-chain…",
	"waiting":        "Esperando el pago de {sats}. Esta factura caduca en {time}.",
	"pendingExpired": "Esta factura ha caducado. Si la pagaste, espera aquí mientras se comprueba el pago; si no, obtén una factura nueva.",
	"processing":     "Pago recibido, pendiente de confirmación. Puedes cerrar esta página; el acceso empieza cuando se confirme.",
	"paidPermanent":  "Pagado. Esta clave pública ya puede publicar en el relay.",
	"paidUntil":      "Pagado. Esta clave pública puede publicar en el relay hasta el {date}.",
	"expired":        "Esta factura caducó sin recibir ningún pago.",
	"unreachable":    "El relay no responde en este momento; seguimos intentándolo.",
	"noExtensionKey": "La extensión no compartió ninguna clave pública.",

	"invalidKey":     "No es una npub ni una clave pública hexadecimal válida.",
	"notTaking":      "Este relay no acepta pagos en este momento.",
	"badRequest":     "Solicitud incorrecta.",
	"tooManyFromYou": "Demasiadas facturas desde tu dirección. Inténtalo de nuevo en unos minutos.",
	"tooManyWaiting": "Hay demasiados registros esperando el pago. Inténtalo de nuevo en unos minutos.",
	"paymentsDown":   "Los pagos no están disponibles en este momento. Inténtalo de nuevo más tarde.",
	"unknownInvoice": "Factura desconocida.",
	"noOnchain":      "Esta factura no se puede pagar on-chain.",
	"onchainDown":    "El pago on-chain no está disponible en este momento. Paga con Lightning o inténtalo de nuevo más tarde.",
	"memo":           "Acceso de escritura a {relay} para {key}",
	"memoNoName":     "Acceso de escritura al relay para {key}",
}

var messagesDE = map[string]string{
	"title":          "Schreibzugriff",
	"titleNamed":     "{name}: Schreibzugriff",
	"nostrRelay":     "Nostr-Relay",
	"open":           "Gerade kann jeder auf diesem Relay veröffentlichen, es gibt also nichts zu bezahlen.",
	"notTakingRetry": "Dieses Relay nimmt gerade keine Zahlungen an. Versuche es später noch einmal.",
	"sats":           "{n} sats",
	"forDays":        "für {n} Tage Schreibzugriff",
	"forDaysOne":     "für {n} Tag Schreibzugriff",
	"forPermanent":   "für dauerhaften Schreibzugriff",
	"readingFree":    "Lesen ist kostenlos. Nach einer Zahlung kann der öffentliche Schlüssel unten auf diesem Relay veröffentlichen.",
	"yourKey":        "Dein öffentlicher Schlüssel",
	"keyPlaceholder": "Füge deinen npub1… ein",
	"useExtension":   "Erweiterung verwenden",
	"forKey":         "Für",
	"lightning":      "Lightning",
	"onchain":        "On-Chain",
	"lnInvoice":      "Lightning-Rechnung",
	"btcAddress":     "Bitcoin-Adresse",
	"openWallet":     "In Wallet öffnen",
	"copyInvoice":    "Rechnung kopieren",
	"copyAddress":    "Adresse kopieren",
	"copy":           "Kopieren",
	"copied":         "Kopiert",
	"addRelay":       "Füge dieses Relay deinem Nostr-Client hinzu, um hier zu veröffentlichen:",
	"addRelayNoURL":  "Füge dieses Relay deinem Nostr-Client hinzu, um hier zu veröffentlichen.",
	"relayAddress":   "Relay-Adresse",
	"newInvoice":     "Neue Rechnung anfordern",
	"back":           "Zurück",
	"done":           "Fertig",
	"noscript":       "Diese Seite braucht JavaScript, um eine Rechnung zu erstellen.",
	"terms":          "Nutzungsbedingungen",
	"privacy":        "Datenschutz",
	"contact":        "Kontakt:",
	"language":       "Sprache",

	"badKey":         "Gib einen npub (npub1…) oder einen 64-stelligen öffentlichen Hex-Schlüssel ein.",
	"error":          "Etwas ist schiefgelaufen. Versuche es noch einmal.",
	"checking":       "Wird geprüft…",
	"blocked":        "Dieser öffentliche Schlüssel darf auf diesem Relay nicht veröffentlichen.",
	"permanent":      "Dieser öffentliche Schlüssel hat bereits dauerhaften Schreibzugriff.",
	"activeUntil":    "Dieser öffentliche Schlüssel kann bis zum {date} veröffentlichen.",
	"payAddDays":     "{sats} zahlen, um {n} Tage hinzuzufügen",
	"payAddDaysOne":  "{sats} zahlen, um {n} Tag hinzuzufügen",
	"payPermanent":   "{sats} für dauerhaften Zugang zahlen",
	"pay":            "{sats} zahlen",
	"memberExpired":  "Der Schreibzugriff dieses öffentlichen Schlüssels ist abgelaufen.",
	"notMember":      "Dieser öffentliche Schlüssel ist noch nicht angemeldet.",
	"creating":       "Rechnung wird erstellt…",
	"gettingAddress": "On-Chain-Adresse wird abgerufen…",
	"waiting":        "Warte auf die Zahlung von {sats}. Diese Rechnung läuft in {time} ab.",
	"pendingExpired": "Diese Rechnung ist abgelaufen. Wenn du sie bezahlt hast, warte hier, während die Zahlung geprüft wird; andernfalls fordere eine neue Rechnung an.",
	"processing":     "Zahlung erhalten, sie wartet auf Bestätigung. Du kannst diese Seite schließen; der Zugang beginnt, sobald sie bestätigt ist.",
	"paidPermanent":  "Bezahlt. Dieser öffentliche Schlüssel kann jetzt auf dem Relay veröffentlichen.",
	"paidUntil":      "Bezahlt. Dieser öffentliche Schlüssel kann bis zum {date} auf dem Relay veröffentlichen.",
	"expired":        "Diese Rechnung ist ohne Zahlung abgelaufen.",
	"unreachable":    "Das Relay antwortet gerade nicht; es wird weiter versucht.",
	"noExtensionKey": "Die Erweiterung hat keinen öffentlichen Schlüssel geteilt.",

	"invalidKey":     "Das ist kein gültiger npub oder öffentlicher Hex-Schlüssel.",
	"notTaking":      "Dieses Relay nimmt gerade keine Zahlungen an.",
	"badRequest":     "Ungültige Anfrage.",
	"tooManyFromYou": "Zu viele Rechnungen von deiner Adresse. Versuche es in ein paar Minuten noch einmal.",
	"tooManyWaiting": "Zu viele Anmeldungen warten auf Zahlung. Versuche es in ein paar Minuten noch einmal.",
	"paymentsDown":   "Zahlungen sind gerade nicht möglich. Versuche es später noch einmal.",
	"unknownInvoice": "Unbekannte Rechnung.",
	"noOnchain":      "Diese Rechnung kann nicht on-chain bezahlt werden.",
	"onchainDown":    "On-Chain-Zahlungen sind gerade nicht möglich. Zahle mit Lightning oder versuche es später noch einmal.",
	"memo":           "Schreibzugriff auf {relay} für {key}",
	"memoNoName":     "Relay-Schreibzugriff für {key}",
}

var messagesPL = map[string]string{
	"title":          "Dostęp do zapisu",
	"titleNamed":     "{name}: dostęp do zapisu",
	"nostrRelay":     "Przekaźnik Nostr",
	"open":           "Obecnie każdy może publikować w tym przekaźniku, więc nie ma za co płacić.",
	"notTakingRetry": "Ten przekaźnik nie przyjmuje teraz płatności. Spróbuj ponownie później.",
	"sats":           "{n} sats",
	"forDays":        "za {n} dni dostępu do zapisu",
	"forDaysOne":     "za {n} dzień dostępu do zapisu",
	"forPermanent":   "za stały dostęp do zapisu",
	"readingFree":    "Czytanie jest bezpłatne. Po opłaceniu klucz publiczny podany poniżej może publikować w tym przekaźniku.",
	"yourKey":        "Twój klucz publiczny",
	"keyPlaceholder": "Wklej swój npub1…",
	"useExtension":   "Użyj rozszerzenia",
	"forKey":         "Dla",
	"lightning":      "Lightning",
	"onchain":        "On-chain",
	"lnInvoice":      "Faktura Lightning",
	"btcAddress":     "Adres Bitcoin",
	"openWallet":     "Otwórz w portfelu",
	"copyInvoice":    "Kopiuj fakturę",
	"copyAddress":    "Kopiuj adres",
	"copy":           "Kopiuj",
	"copied":         "Skopiowano",
	"addRelay":       "Dodaj ten przekaźnik do swojego klienta Nostr, aby tu publikować:",
	"addRelayNoURL":  "Dodaj ten przekaźnik do swojego klienta Nostr, aby tu publikować.",
	"relayAddress":   "Adres przekaźnika",
	"newInvoice":     "Utwórz nową fakturę",
	"back":           "Wstecz",
	"done":           "Gotowe",
	"noscript":       "Ta strona potrzebuje JavaScriptu, aby utworzyć fakturę.",
	"terms":          "Regulamin",
	"privacy":        "Prywatność",
	"contact":        "Kontakt:",
	"language":       "Język",

	"badKey":         "Wpisz npub (npub1…) lub 64-znakowy szesnastkowy klucz publiczny.",
	"error":          "Coś poszło nie tak. Spróbuj ponownie.",
	"checking":       "Sprawdzanie…",
	"blocked":        "Ten klucz publiczny nie może publikować w tym przekaźniku.",
	"permanent":      "Ten klucz publiczny ma już stały dostęp do zapisu.",
	"activeUntil":    "Ten klucz publiczny może publikować do {date}.",
	"payAddDays":     "Zapłać {sats}, aby dodać {n} dni",
	"payAddDaysOne":  "Zapłać {sats}, aby dodać {n} dzień",
	"payPermanent":   "Zapłać {sats} za stały dostęp",
	"pay":            "Zapłać {sats}",
	"memberExpired":  "Dostęp do zapisu dla tego klucza publicznego wygasł.",
	"notMember":      "Ten klucz publiczny nie jest jeszcze zarejestrowany.",
	"creating":       "Tworzenie faktury…",
	"gettingAddress": "Pobieranie adresu on-chain…",
	"waiting":        "Oczekiwanie na płatność {sats}. Ta faktura wygaśnie za {time}.",
	"pendingExpired": "Ta faktura wygasła. Jeśli została opłacona, zaczekaj tutaj na sprawdzenie płatności; jeśli nie, utwórz nową fakturę.",
	"processing":     "Płatność otrzymana, czeka na potwierdzenie. Możesz zamknąć tę stronę; dostęp zacznie się po potwierdzeniu.",
	"paidPermanent":  "Opłacono. Ten klucz publiczny może teraz publikować w przekaźniku.",
	"paidUntil":      "Opłacono. Ten klucz publiczny może publikować w przekaźniku do {date}.",
	"expired":        "Ta faktura wygasła bez płatności.",
	"unreachable":    "Przekaźnik teraz nie odpowiada; nadal próbujemy.",
	"noExtensionKey": "Rozszerzenie nie udostępniło klucza publicznego.",

	"invalidKey":     "To nie jest prawidłowy npub ani szesnastkowy klucz publiczny.",
	"notTaking":      "Ten przekaźnik nie przyjmuje teraz płatności.",
	"badRequest":     "Nieprawidłowe żądanie.",
	"tooManyFromYou": "Zbyt wiele faktur z Twojego adresu. Spróbuj ponownie za kilka minut.",
	"tooManyWaiting": "Zbyt wiele rejestracji czeka na płatność. Spróbuj ponownie za kilka minut.",
	"paymentsDown":   "Płatności są teraz niedostępne. Spróbuj ponownie później.",
	"unknownInvoice": "Nieznana faktura.",
	"noOnchain":      "Tej faktury nie można opłacić on-chain.",
	"onchainDown":    "Płatność on-chain jest teraz niedostępna. Zapłać przez Lightning lub spróbuj ponownie później.",
	"memo":           "Dostęp do zapisu w {relay} dla {key}",
	"memoNoName":     "Dostęp do zapisu w przekaźniku dla {key}",
}

var messagesFR = map[string]string{
	"title":          "Accès en écriture",
	"titleNamed":     "{name} : accès en écriture",
	"nostrRelay":     "Relais Nostr",
	"open":           "Tout le monde peut publier sur ce relais en ce moment : il n'y a donc rien à payer.",
	"notTakingRetry": "Ce relais n'accepte pas de paiements en ce moment. Réessayez plus tard.",
	"sats":           "{n} sats",
	"forDays":        "pour {n} jours d'accès en écriture",
	"forDaysOne":     "pour {n} jour d'accès en écriture",
	"forPermanent":   "pour un accès en écriture permanent",
	"readingFree":    "La lecture est gratuite. Le paiement permet à la clé publique ci-dessous de publier sur ce relais.",
	"yourKey":        "Votre clé publique",
	"keyPlaceholder": "Collez votre npub1…",
	"useExtension":   "Utiliser l'extension",
	"forKey":         "Pour",
	"lightning":      "Lightning",
	"onchain":        "On-chain",
	"lnInvoice":      "Facture Lightning",
	"btcAddress":     "Adresse Bitcoin",
	"openWallet":     "Ouvrir dans le portefeuille",
	"copyInvoice":    "Copier la facture",
	"copyAddress":    "Copier l'adresse",
	"copy":           "Copier",
	"copied":         "Copié",
	"addRelay":       "Ajoutez ce relais à votre client Nostr pour publier ici :",
	"addRelayNoURL":  "Ajoutez ce relais à votre client Nostr pour publier ici.",
	"relayAddress":   "Adresse du relais",
	"newInvoice":     "Obtenir une nouvelle facture",
	"back":           "Retour",
	"done":           "Terminé",
	"noscript":       "Cette page a besoin de JavaScript pour créer une facture.",
	"terms":          "Conditions",
	"privacy":        "Confidentialité",
	"contact":        "Contact :",
	"language":       "Langue",

	"badKey":         "Saisissez une npub (npub1…) ou une clé publique hexadécimale de 64 caractères.",
	"error":          "Une erreur s'est produite. Réessayez.",
	"checking":       "Vérification…",
	"blocked":        "Cette clé publique ne peut pas publier sur ce relais.",
	"permanent":      "Cette clé publique a déjà un accès en écriture permanent.",
	"activeUntil":    "Cette clé publique peut publier jusqu'au {date}.",
	"payAddDays":     "Payer {sats} pour ajouter {n} jours",
	"payAddDaysOne":  "Payer {sats} pour ajouter {n} jour",
	"payPermanent":   "Payer {sats} pour un accès permanent",
	"pay":            "Payer {sats}",
	"memberExpired":  "L'accès en écriture de cette clé publique a expiré.",
	"notMember":      "Cette clé publique n'est pas encore inscrite.",
	"creating":       "Création de la facture…",
	"gettingAddress": "Obtention d'une adresse on-chain…",
	"waiting":        "En attente du paiement de {sats}. Cette facture expire dans {time}.",
	"pendingExpired": "Cette facture a expiré. Si vous l'avez payée, patientez ici pendant la vérification du paiement ; sinon, obtenez une nouvelle facture.",
	"processing":     "Paiement reçu, en attente de confirmation. Vous pouvez fermer cette page ; l'accès commence dès qu'il est confirmé.",
	"paidPermanent":  "Payé. Cette clé publique peut maintenant publier sur le relais.",
	"paidUntil":      "Payé. Cette clé publique peut publier sur le relais jusqu'au {date}.",
	"expired":        "Cette facture a expiré sans paiement.",
	"unreachable":    "Le relais ne répond pas pour le moment ; nouvelle tentative en cours.",
	"noExtensionKey": "L'extension n'a partagé aucune clé publique.",

	"invalidKey":     "Ce n'est pas une npub ni une clé publique hexadécimale valide.",
	"notTaking":      "Ce relais n'accepte pas de paiements en ce moment.",
	"badRequest":     "Requête invalide.",
	"tooManyFromYou": "Trop de factures depuis votre adresse. Réessayez dans quelques minutes.",
	"tooManyWaiting": "Trop d'inscriptions attendent un paiement. Réessayez dans quelques minutes.",
	"paymentsDown":   "Les paiements sont indisponibles pour le moment. Réessayez plus tard.",
	"unknownInvoice": "Facture inconnue.",
	"noOnchain":      "Cette facture ne peut pas être payée on-chain.",
	"onchainDown":    "Le paiement on-chain est indisponible pour le moment. Payez avec Lightning, ou réessayez plus tard.",
	"memo":           "Accès en écriture à {relay} pour {key}",
	"memoNoName":     "Accès en écriture au relais pour {key}",
}
