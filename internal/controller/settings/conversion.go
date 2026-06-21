/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package settings

import (
	"fmt"
	"strconv"

	"github.com/AYDEV-FR/ctfd-crossplane-provider/apis/resources/v1alpha1"
)

// desiredConfig builds the map of CTFd configuration keys to apply from the
// desired parameters. Only fields the user set are included, so a PATCH leaves
// every other CTFd setting untouched. mailPassword is the resolved SMTP
// password (empty if none).
func desiredConfig(p v1alpha1.SettingsParameters, mailPassword string) map[string]any {
	m := map[string]any{}
	set := func(key string, v *string) {
		if v != nil {
			m[key] = *v
		}
	}
	setBool := func(key string, v *bool) {
		if v != nil {
			m[key] = *v
		}
	}
	setInt := func(key string, v *int) {
		if v != nil {
			m[key] = *v
		}
	}

	set("ctf_name", p.Name)
	set("ctf_description", p.Description)
	set("ctf_theme", p.Theme)
	set("theme_header", p.ThemeHeader)
	set("theme_footer", p.ThemeFooter)
	set("theme_settings", p.ThemeSettings)
	set("default_locale", p.DefaultLocale)
	set("challenge_visibility", p.ChallengeVisibility)
	set("account_visibility", p.AccountVisibility)
	set("score_visibility", p.ScoreVisibility)
	set("registration_visibility", p.RegistrationVisibility)
	set("start", p.Start)
	set("end", p.End)
	set("freeze", p.Freeze)
	setBool("paused", p.Paused)
	set("user_mode", p.UserMode)
	setInt("team_size", p.TeamSize)
	setBool("verify_emails", p.VerifyEmails)
	setBool("name_changes", p.NameChanges)
	set("registration_code", p.RegistrationCode)
	set("challenge_ratings", p.ChallengeRatings)
	setBool("hints_free_public_access", p.HintsFreePublicAccess)
	setBool("html_sanitization", p.HTMLSanitization)
	set("robots_txt", p.RobotsTxt)
	set("tos_url", p.TOSURL)
	set("tos_text", p.TOSText)
	set("privacy_url", p.PrivacyURL)
	set("privacy_text", p.PrivacyText)

	if mp := p.Mail; mp != nil {
		set("mail_server", mp.Server)
		set("mail_port", mp.Port)
		set("mail_username", mp.Username)
		setBool("mail_tls", mp.TLS)
		setBool("mail_ssl", mp.SSL)
		setBool("mail_useauth", mp.UseAuth)
		if mailPassword != "" {
			m["mail_password"] = mailPassword
		}
	}

	return m
}

// configString renders a desired config value the way CTFd stores it in the
// /configs response (always a string), so observed and desired can be compared.
// CTFd stores booleans as "1"/"0", not "true"/"false".
func configString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return "0"
	case int:
		return strconv.Itoa(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// isUpToDate reports whether every desired config key matches the value
// observed in CTFd. The mail password is not compared (CTFd never returns it).
func isUpToDate(desired map[string]any, observed map[string]string) bool {
	for k, v := range desired {
		if k == "mail_password" {
			continue
		}
		if observed[k] != configString(v) {
			return false
		}
	}
	return true
}
