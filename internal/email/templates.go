// internal/email/templates.go

package email

import (
	"bytes"
	"html/template"

	"task-forge/internal/dto"
)

const invitationEmailTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>TaskForge Invitation</title>
</head>

<body style="
	margin: 0;
	padding: 0;
	background-color: #f4f7fb;
	font-family: Arial, Helvetica, sans-serif;
">

<table width="100%" cellpadding="0" cellspacing="0" style="padding: 40px 20px;">
	<tr>
		<td align="center">

			<table width="600" cellpadding="0" cellspacing="0" style="
				max-width: 600px;
				background: #ffffff;
				border-radius: 12px;
				overflow: hidden;
			">

				<!-- Header -->
				<tr>
					<td style="
						background-color: #2563eb;
						padding: 28px;
						text-align: center;
					">
						<h1 style="
							margin: 0;
							color: #ffffff;
							font-size: 28px;
						">
							TaskForge
						</h1>
					</td>
				</tr>

				<!-- Content -->
				<tr>
					<td style="padding: 40px;">

						<h2 style="
							margin-top: 0;
							color: #111827;
							font-size: 24px;
						">
							You're invited to join a team
						</h2>

						<p style="
							color: #4b5563;
							font-size: 16px;
							line-height: 1.6;
						">
							<strong>{{ .InviterName }}</strong>
							has invited you to join the team
							<strong>{{ .TeamName }}</strong>.
						</p>

						<p style="
							color: #4b5563;
							font-size: 16px;
							line-height: 1.6;
						">
							You will join the team with the following role:
							<strong>{{ .Role }}</strong>.
						</p>

						<div style="
							text-align: center;
							margin: 32px 0;
						">
							<a href="{{ .InviteURL }}" style="
								display: inline-block;
								padding: 14px 28px;
								background-color: #2563eb;
								color: #ffffff;
								text-decoration: none;
								border-radius: 8px;
								font-weight: bold;
								font-size: 16px;
							">
								Join Team
							</a>
						</div>

						<p style="
							color: #6b7280;
							font-size: 14px;
							line-height: 1.5;
						">
							If you were not expecting this invitation,
							you can safely ignore this email.
						</p>

					</td>
				</tr>

				<!-- Footer -->
				<tr>
					<td style="
						padding: 20px 40px;
						background-color: #f9fafb;
						color: #9ca3af;
						font-size: 12px;
						text-align: center;
					">
						This is an automated message from TaskForge.
					</td>
				</tr>

			</table>

		</td>
	</tr>
</table>

</body>
</html>
`

func buildInvitationTemplate(data dto.InvitationEmailData) (string, error) {
	tmpl, err := template.New("invitation").Parse(invitationEmailTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
