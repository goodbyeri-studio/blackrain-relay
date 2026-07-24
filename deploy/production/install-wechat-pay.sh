#!/usr/bin/env bash
set -Eeuo pipefail

BUNDLE_DIR="${1:-}"
ENV_FILE="${WECHAT_PAY_ENV_FILE:-/etc/meimei-api/production.env}"
SECRET_DIR="${WECHAT_PAY_SECRET_DIR:-/etc/meimei-api/secrets/wechatpay}"

fail() {
  printf '[wechat-pay-config] ERROR: %s\n' "$*" >&2
  exit 1
}

if [[ "${EUID:-$(id -u)}" -ne 0 && "$ENV_FILE" == /etc/* ]]; then
  exec sudo -n -- "$0" "$@"
fi

[[ -n "$BUNDLE_DIR" && -d "$BUNDLE_DIR" ]] || fail 'configuration bundle is missing'
[[ -f "$ENV_FILE" ]] || fail "production environment file is missing: $ENV_FILE"

PAYMENT_ENV="$BUNDLE_DIR/wechat-pay.env"
MERCHANT_KEY="$BUNDLE_DIR/merchant-private-key.pem"
PUBLIC_KEY="$BUNDLE_DIR/wechatpay-public-key.pem"
temporary_env=''

cleanup() {
  [[ -z "$temporary_env" ]] || rm -f "$temporary_env"
  rm -f "$PAYMENT_ENV" "$MERCHANT_KEY" "$PUBLIC_KEY"
  rmdir "$BUNDLE_DIR" 2>/dev/null || true
}
trap cleanup EXIT

[[ -s "$PAYMENT_ENV" ]] || fail 'WeChat Pay environment bundle is empty'
[[ -s "$MERCHANT_KEY" ]] || fail 'merchant private key is empty'
[[ -s "$PUBLIC_KEY" ]] || fail 'WeChat Pay public key is empty'

install -m 700 -d "$SECRET_DIR"
install -m 600 "$MERCHANT_KEY" "$SECRET_DIR/merchant-private-key.pem"
install -m 600 "$PUBLIC_KEY" "$SECRET_DIR/wechatpay-public-key.pem"

temporary_env=$(mktemp "${ENV_FILE}.tmp.XXXXXX")
awk '!/^WECHAT_PAY_/' "$ENV_FILE" > "$temporary_env"
cat "$PAYMENT_ENV" >> "$temporary_env"
chmod --reference="$ENV_FILE" "$temporary_env"
chown --reference="$ENV_FILE" "$temporary_env"
mv "$temporary_env" "$ENV_FILE"
temporary_env=''

printf '[wechat-pay-config] production configuration installed\n'
