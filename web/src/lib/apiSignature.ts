import { createHmac } from "node:crypto";

type SignaturePayloadInput = {
  method: string;
  path: string;
  rawQuery: string;
  timestamp: string;
};

type SignedHeadersInput = SignaturePayloadInput & {
  clientId: string;
  clientSecret: string;
};

export function signaturePayload({
  method,
  path,
  rawQuery,
  timestamp,
}: SignaturePayloadInput): string {
  return `${method}\n${path}\n${rawQuery}\n${timestamp}`;
}

export function createSignedHeaders(
  input: SignedHeadersInput,
): Record<string, string> {
  const signature = createHmac("sha256", input.clientSecret)
    .update(signaturePayload(input))
    .digest("hex");

  return {
    "X-Client-ID": input.clientId,
    "X-Timestamp": input.timestamp,
    "X-Signature": signature,
  };
}
