import { Alert } from "../ui";
import type { CertInfo } from "../api/client";

interface Props {
  cert: CertInfo | null;
  ownershipVerified: boolean;
  isIssuing?: boolean;
}

export function CertStatusCard({ cert, ownershipVerified, isIssuing }: Props) {
  if (cert !== null) {
    return <Alert variant="success">Certificate is issued and active.</Alert>;
  }

  if (!ownershipVerified) {
    return (
      <Alert variant="neutral">Certificate will be issued once domain ownership is verified.</Alert>
    );
  }

  if (isIssuing) {
    return <Alert variant="warning">Certificate issuance in progress…</Alert>;
  }

  return (
    <Alert variant="neutral">
      Not yet issued. It may be issued automatically, or use the button below to trigger it
      manually.
    </Alert>
  );
}
