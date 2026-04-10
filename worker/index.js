const GITHUB_REPO = "evatt-labs/kraai-cli";

const INSTALL_SCRIPT = `#!/bin/sh
set -e

REPO="evatt-labs/kraai-cli"
BINARY="kraai"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)   OS=linux ;;
  Darwin*)  OS=darwin ;;
  *)        echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect arch
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *)             echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Fetch latest release version from GitHub
VERSION="__VERSION__"

ARCHIVE="kraai_\${VERSION}_\${OS}_\${ARCH}.tar.gz"
URL="https://github.com/\${REPO}/releases/download/v\${VERSION}/\${ARCHIVE}"
CHECKSUM_URL="https://github.com/\${REPO}/releases/download/v\${VERSION}/kraai_\${VERSION}_checksums.txt"

echo "Installing Kraai CLI \${VERSION} (\${OS}/\${ARCH})..."

# Download to temp dir
TMP="\$(mktemp -d)"
trap 'rm -rf "\$TMP"' EXIT

curl -fsSL "\$URL" -o "\$TMP/\${ARCHIVE}"

# Verify checksum if shasum/sha256sum is available
if command -v sha256sum >/dev/null 2>&1; then
  EXPECTED="\$(curl -fsSL "\$CHECKSUM_URL" | grep "\${ARCHIVE}" | awk '{print \$1}')"
  ACTUAL="\$(sha256sum "\$TMP/\${ARCHIVE}" | awk '{print \$1}')"
  if [ "\$EXPECTED" != "\$ACTUAL" ]; then
    echo "Checksum mismatch — aborting." >&2
    exit 1
  fi
elif command -v shasum >/dev/null 2>&1; then
  EXPECTED="\$(curl -fsSL "\$CHECKSUM_URL" | grep "\${ARCHIVE}" | awk '{print \$1}')"
  ACTUAL="\$(shasum -a 256 "\$TMP/\${ARCHIVE}" | awk '{print \$1}')"
  if [ "\$EXPECTED" != "\$ACTUAL" ]; then
    echo "Checksum mismatch — aborting." >&2
    exit 1
  fi
fi

tar -xzf "\$TMP/\${ARCHIVE}" -C "\$TMP"

# Install to /usr/local/bin if writable, otherwise ~/.local/bin
if [ -w /usr/local/bin ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ "\$(id -u)" -eq 0 ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="\$HOME/.local/bin"
  mkdir -p "\$INSTALL_DIR"
fi

mv "\$TMP/\${BINARY}" "\$INSTALL_DIR/\${BINARY}"
chmod +x "\$INSTALL_DIR/\${BINARY}"

echo ""
echo "  kraai \${VERSION} installed to \${INSTALL_DIR}/kraai"
echo ""

# Warn if install dir is not in PATH
case ":\$PATH:" in
  *":\${INSTALL_DIR}:"*) ;;
  *) echo "  Note: \${INSTALL_DIR} is not in your PATH. Add it to your shell profile." ;;
esac

echo "  Run 'kraai login' to get started."
echo ""
`;

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    // Health check
    if (url.pathname === "/health") {
      return new Response("ok", { status: 200 });
    }

    // Fetch the latest release version from GitHub
    let version;
    try {
      const resp = await fetch(
        `https://api.github.com/repos/${GITHUB_REPO}/releases/latest`,
        { headers: { "User-Agent": "kraai-get-worker" } }
      );
      if (!resp.ok) throw new Error(`GitHub API ${resp.status}`);
      const data = await resp.json();
      version = data.tag_name.replace(/^v/, "");
    } catch (err) {
      return new Response(`Failed to fetch latest release: ${err.message}\n`, {
        status: 502,
        headers: { "Content-Type": "text/plain" },
      });
    }

    const script = INSTALL_SCRIPT.replace("__VERSION__", version);

    return new Response(script, {
      status: 200,
      headers: {
        "Content-Type": "text/plain; charset=utf-8",
        "Cache-Control": "public, max-age=300",
        "X-Kraai-Version": version,
      },
    });
  },
};
