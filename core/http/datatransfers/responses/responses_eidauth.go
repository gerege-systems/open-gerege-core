// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import eidauthuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/eidauth"

// EIDAuthStartResponse нь /eid-auth/start(-id)-ийн хариу.
type EIDAuthStartResponse struct {
	SessionID        string `json:"session_id"`
	DeviceLinkURL    string `json:"device_link_url,omitempty"`
	VerificationCode string `json:"verification_code"`
	ExpiresAt        string `json:"expires_at"`
}

// FromEIDAuthStart нь usecase-ийн үр дүнг DTO рүү буулгана.
func FromEIDAuthStart(r eidauthuc.StartResponse) EIDAuthStartResponse {
	return EIDAuthStartResponse{
		SessionID:        r.SessionID,
		DeviceLinkURL:    r.DeviceLinkURL,
		VerificationCode: r.VerificationCode,
		ExpiresAt:        r.ExpiresAt,
	}
}

// EIDAuthIdentityResponse нь eID-ээр баталгаажсан иргэний мэдээлэл.
type EIDAuthIdentityResponse struct {
	CivilID        string `json:"civil_id"`
	NationalID     string `json:"national_id,omitempty"`
	GivenName      string `json:"given_name,omitempty"`
	Surname        string `json:"surname,omitempty"`
	GivenNameEn    string `json:"given_name_en,omitempty"`
	SurnameEn      string `json:"surname_en,omitempty"`
	FullName       string `json:"full_name,omitempty"`
	KYCLevel       string `json:"kyc_level,omitempty"`
	DocumentNumber string `json:"document_number,omitempty"`
}

// EIDAuthPollResponse нь /eid-auth/poll-ийн хариу. Identity нь зөвхөн COMPLETE
// төлөвт ирнэ.
type EIDAuthPollResponse struct {
	State    string                   `json:"state"`
	Identity *EIDAuthIdentityResponse `json:"identity,omitempty"`
}

// FromEIDAuthPoll нь poll-ийн үр дүнг DTO рүү буулгана.
func FromEIDAuthPoll(r eidauthuc.PollResponse) EIDAuthPollResponse {
	out := EIDAuthPollResponse{State: r.State}
	if r.Identity != nil {
		out.Identity = &EIDAuthIdentityResponse{
			CivilID:        r.Identity.CivilID,
			NationalID:     r.Identity.NationalID,
			GivenName:      r.Identity.GivenName,
			Surname:        r.Identity.Surname,
			GivenNameEn:    r.Identity.GivenNameEn,
			SurnameEn:      r.Identity.SurnameEn,
			FullName:       r.Identity.FullName,
			KYCLevel:       r.Identity.KYCLevel,
			DocumentNumber: r.Identity.DocumentNumber,
		}
	}
	return out
}
