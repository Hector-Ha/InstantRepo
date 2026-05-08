const secretAssignmentPattern =
  /\b([A-Z0-9_]*(?:SECRET|TOKEN|PASSWORD|PASSWD|API_KEY|APIKEY|PRIVATE_KEY|CLIENT_SECRET|ACCESS_KEY)[A-Z0-9_]*)\s*=\s*("[^"]*"|'[^']*'|[^\s]+)/gi;
const secretKeyValuePattern =
  /\b((?:secret|token|password|passwd|api[_-]?key|private[_-]?key|client[_-]?secret)\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s]+)/gi;
const secretFlagPattern =
  /(--?(?:secret|token|password|api-key|apikey|private-key|client-secret)\s+)([^\s]+)/gi;
const bearerTokenPattern = /\b(Bearer)\s+[A-Za-z0-9._~+/-]+=*/gi;
const urlCredentialPattern = /([a-z][a-z0-9+.-]*:\/\/[^:\s/@]+:)[^@\s]+@/gi;

export function redactLikelySecrets(input: string) {
  if (input === "") {
    return input;
  }

  return input
    .replace(secretAssignmentPattern, "$1=[REDACTED]")
    .replace(secretKeyValuePattern, "$1[REDACTED]")
    .replace(secretFlagPattern, "$1[REDACTED]")
    .replace(bearerTokenPattern, "$1 [REDACTED]")
    .replace(urlCredentialPattern, "$1[REDACTED]@");
}
