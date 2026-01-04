# Security Notice for Configuration Examples

## ⚠️ IMPORTANT: Example Credentials Only

All files in the `config-templates/` directory contain **EXAMPLE CREDENTIALS FOR DEMONSTRATION PURPOSES ONLY**.

### What's Included

- **PEM files (certificates and keys)**: Self-signed certificates for testing
- **secrets.json files**: Example password hashes
- **Configuration files**: Sample settings

### ⚠️ CRITICAL: Do NOT Use These in Production

These example files are:
- ✅ Safe to commit to version control (they're examples)
- ✅ Intended for learning and testing
- ❌ **NOT secure for production use**
- ❌ **NOT real secrets** (they're publicly visible)

### Before Deploying to Production

1. **Generate your own certificates**:
   ```bash
   # See individual README files for certificate generation commands
   openssl req -x509 -newkey rsa:4096 -keyout server-key.pem -out server.pem -days 365
   ```

2. **Generate your own passwords**:
   ```bash
   go run cmd/generatepasswords/generatepasswords.go -digest sha512 -iterations 100 -password YOUR_SECURE_PASSWORD
   ```

3. **Never commit real secrets**:
   - Add your production config files to `.gitignore`
   - Use environment variables or secret management systems
   - Rotate credentials regularly

### For GitHub Secret Scanning

This directory is marked as documentation in `.gitattributes` to prevent false-positive security warnings. These are intentionally public example credentials.
