package service

import (
	"fmt"
	"html"

	"coin-alert/internal/email"
)

// normalizeEmailLocale keeps the email language to one we have copy for, defaulting to pt-BR.
func normalizeEmailLocale(locale string) string {
	switch locale {
	case "en", "es", "pt":
		return locale
	default:
		return "pt"
	}
}

func passwordResetEmail(locale string, link string) email.Message {
	switch normalizeEmailLocale(locale) {
	case "en":
		return email.Message{
			Subject:  "Coin Hub — reset your password",
			TextBody: "You asked to reset your Coin Hub password. Open the link below (valid for 1 hour):\n\n" + link + "\n\nIf this wasn't you, just ignore this email — your password stays the same.",
			HTMLBody: brandedEmailHTML("Reset your password", "You asked to reset your Coin Hub password. Click the button below to choose a new one. The link is valid for 1 hour.", "Reset password", link, "If this wasn't you, just ignore this email — your password stays the same."),
		}
	case "es":
		return email.Message{
			Subject:  "Coin Hub — restablecer tu contraseña",
			TextBody: "Solicitaste restablecer tu contraseña de Coin Hub. Abre el enlace de abajo (válido por 1 hora):\n\n" + link + "\n\nSi no fuiste tú, ignora este correo — tu contraseña no cambia.",
			HTMLBody: brandedEmailHTML("Restablecer tu contraseña", "Solicitaste restablecer tu contraseña de Coin Hub. Haz clic en el botón para elegir una nueva. El enlace es válido por 1 hora.", "Restablecer contraseña", link, "Si no fuiste tú, ignora este correo — tu contraseña no cambia."),
		}
	default:
		return email.Message{
			Subject:  "Coin Hub — redefinição de senha",
			TextBody: "Você pediu para redefinir sua senha no Coin Hub. Abra o link abaixo (válido por 1 hora):\n\n" + link + "\n\nSe não foi você, ignore este e-mail — sua senha continua a mesma.",
			HTMLBody: brandedEmailHTML("Redefinir sua senha", "Você pediu para redefinir sua senha no Coin Hub. Clique no botão para escolher uma nova. O link é válido por 1 hora.", "Redefinir senha", link, "Se não foi você, ignore este e-mail — sua senha continua a mesma."),
		}
	}
}

func emailVerificationEmail(locale string, link string) email.Message {
	switch normalizeEmailLocale(locale) {
	case "en":
		return email.Message{
			Subject:  "Coin Hub — confirm your email",
			TextBody: "Welcome to Coin Hub! Confirm your email by opening the link below (valid for 24 hours):\n\n" + link + "\n\nIf you didn't create an account, just ignore this email.",
			HTMLBody: brandedEmailHTML("Confirm your email", "Welcome to Coin Hub! Confirm your email address so we know it's really you. The link is valid for 24 hours.", "Confirm email", link, "If you didn't create an account, just ignore this email."),
		}
	case "es":
		return email.Message{
			Subject:  "Coin Hub — confirma tu correo",
			TextBody: "¡Bienvenido a Coin Hub! Confirma tu correo abriendo el enlace de abajo (válido por 24 horas):\n\n" + link + "\n\nSi no creaste una cuenta, ignora este correo.",
			HTMLBody: brandedEmailHTML("Confirma tu correo", "¡Bienvenido a Coin Hub! Confirma tu dirección de correo para que sepamos que eres tú. El enlace es válido por 24 horas.", "Confirmar correo", link, "Si no creaste una cuenta, ignora este correo."),
		}
	default:
		return email.Message{
			Subject:  "Coin Hub — confirme seu e-mail",
			TextBody: "Bem-vindo ao Coin Hub! Confirme seu e-mail abrindo o link abaixo (válido por 24 horas):\n\n" + link + "\n\nSe você não criou uma conta, ignore este e-mail.",
			HTMLBody: brandedEmailHTML("Confirme seu e-mail", "Bem-vindo ao Coin Hub! Confirme seu endereço de e-mail para sabermos que é você mesmo. O link é válido por 24 horas.", "Confirmar e-mail", link, "Se você não criou uma conta, ignore este e-mail."),
		}
	}
}

// brandedEmailHTML renders a simple, inline-styled email matching the warm-dark + gold brand. Links
// and text are HTML-escaped.
func brandedEmailHTML(heading string, paragraph string, buttonLabel string, link string, footer string) string {
	safeHeading := html.EscapeString(heading)
	safeParagraph := html.EscapeString(paragraph)
	safeButton := html.EscapeString(buttonLabel)
	safeLink := html.EscapeString(link)
	safeFooter := html.EscapeString(footer)
	return fmt.Sprintf(`<!doctype html>
<html><body style="margin:0;background:#1a1714;font-family:Segoe UI,Arial,sans-serif;color:#fff9db;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#1a1714;padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="max-width:480px;background:#231f1b;border:1px solid #3a332b;border-radius:14px;padding:32px;">
        <tr><td style="font-size:22px;font-weight:800;color:#ffd43b;padding-bottom:16px;">Coin&nbsp;Hub</td></tr>
        <tr><td style="font-size:18px;font-weight:700;padding-bottom:12px;">%s</td></tr>
        <tr><td style="font-size:15px;line-height:1.6;color:#e9e2cf;padding-bottom:24px;">%s</td></tr>
        <tr><td style="padding-bottom:24px;"><a href="%s" style="display:inline-block;background:#ffd43b;color:#1a1714;font-weight:800;text-decoration:none;padding:12px 22px;border-radius:10px;">%s</a></td></tr>
        <tr><td style="font-size:13px;line-height:1.6;color:#a89f8c;word-break:break-all;">%s<br><br><a href="%s" style="color:#ffd43b;">%s</a></td></tr>
      </table>
    </td></tr>
  </table>
</body></html>`, safeHeading, safeParagraph, safeLink, safeButton, safeFooter, safeLink, safeLink)
}
