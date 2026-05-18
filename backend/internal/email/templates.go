package email

// Templates email. Sengaja inline (string constant) supaya binary
// self-contained tanpa file external. Kalau mau ubah, edit di sini.

const tmplTempPasswordText = `Halo {{.Username}},

Berikut adalah temporary password Anda:

    {{.TempPassword}}

PENTING:
- Password ini hanya untuk login pertama kali.
- Sistem akan meminta Anda mengganti password setelah login.
- Jangan bagikan password ini kepada siapa pun.
- Jika Anda tidak meminta reset password, segera hubungi tim IT.

Salam,
{{.FromName}}

(c) {{.Year}}
`

const tmplTempPasswordHTML = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Temporary Password</title></head>
<body style="font-family: Arial, sans-serif; color:#222; background:#f6f6f6; padding:20px;">
  <table style="max-width:560px;margin:0 auto;background:#fff;border-radius:8px;padding:24px;">
    <tr><td>
      <h2 style="margin-top:0;color:#1f2937;">Reset Password</h2>
      <p>Halo <b>{{.Username}}</b>,</p>
      <p>Berikut adalah <b>temporary password</b> Anda:</p>
      <p style="font-size:18px;background:#f4f4f4;padding:14px 16px;border-radius:6px;font-family:'Courier New',monospace;letter-spacing:1px;">
        {{.TempPassword}}
      </p>
      <p style="background:#fff7ed;border-left:4px solid #f97316;padding:12px 14px;color:#7c2d12;">
        <b>PENTING:</b>
        <ul style="margin:6px 0 0 0;padding-left:18px;">
          <li>Password ini hanya untuk login pertama kali.</li>
          <li>Sistem akan meminta Anda mengganti password setelah login.</li>
          <li>Jangan bagikan password ini kepada siapa pun.</li>
        </ul>
      </p>
      <p>Jika Anda tidak meminta reset password ini, segera hubungi tim IT.</p>
      <p style="color:#6b7280;font-size:13px;">Salam,<br/>{{.FromName}}</p>
      <hr style="border:none;border-top:1px solid #e5e7eb;margin:18px 0;">
      <p style="color:#9ca3af;font-size:12px;text-align:center;">&copy; {{.Year}} {{.FromName}}</p>
    </td></tr>
  </table>
</body>
</html>`

const tmplPasswordChangedText = `Halo {{.Username}},

Password akun Anda baru saja diubah pada {{.Time}}.

Jika ini bukan Anda yang melakukannya, segera hubungi tim IT untuk
mengamankan akun Anda.

Salam,
{{.FromName}}

(c) {{.Year}}
`

const tmplPasswordChangedHTML = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Password Diubah</title></head>
<body style="font-family: Arial, sans-serif; color:#222; background:#f6f6f6; padding:20px;">
  <table style="max-width:560px;margin:0 auto;background:#fff;border-radius:8px;padding:24px;">
    <tr><td>
      <h2 style="margin-top:0;color:#1f2937;">Password Diubah</h2>
      <p>Halo <b>{{.Username}}</b>,</p>
      <p>Password akun Anda baru saja diubah pada <b>{{.Time}}</b>.</p>
      <p style="background:#fef2f2;border-left:4px solid #ef4444;padding:12px 14px;color:#7f1d1d;">
        Jika <b>BUKAN</b> Anda yang melakukannya, segera hubungi tim IT
        untuk mengamankan akun Anda.
      </p>
      <p style="color:#6b7280;font-size:13px;">Salam,<br/>{{.FromName}}</p>
      <hr style="border:none;border-top:1px solid #e5e7eb;margin:18px 0;">
      <p style="color:#9ca3af;font-size:12px;text-align:center;">&copy; {{.Year}} {{.FromName}}</p>
    </td></tr>
  </table>
</body>
</html>`
