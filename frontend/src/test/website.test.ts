import { describe, expect, it } from 'vitest';

import { InboundFormSchema } from '@/schemas/forms/inbound-form';
import { formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';

function input(autoTLS = true) {
  return {
    protocol: 'vless',
    port: 443,
    settings: { clients: [] },
    streamSettings: {
      network: 'xhttp',
      security: 'tls',
      xhttpSettings: { path: '/connect-test/' },
      tlsSettings: { certificates: [], serverName: 'site.example.com' },
      website: { enabled: true, domain: 'site.example.com', autoTLS },
    },
  };
}

describe('built-in website configuration', () => {
  it('can create automatic TLS without supplying a certificate and preserves settings when saved', () => {
    const parsed = InboundFormSchema.parse(input());
    const wire = formValuesToWirePayload(parsed);
    expect(wire.port).toBe(443);
    expect(JSON.parse(wire.streamSettings)).toMatchObject({
      security: 'tls',
      website: { enabled: true, autoTLS: true, domain: 'site.example.com' },
      xhttpSettings: { path: '/connect-test/' },
    });
  });

  it('still requires certificates for manual TLS', () => {
    expect(InboundFormSchema.safeParse(input(false)).success).toBe(false);
  });

  it('rejects root paths and invalid hostnames before saving', () => {
    const root = input();
    root.streamSettings.xhttpSettings.path = '/';
    expect(InboundFormSchema.safeParse(root).success).toBe(false);
    const invalid = input();
    invalid.streamSettings.website.domain = 'https://site.example.com:443';
    expect(InboundFormSchema.safeParse(invalid).success).toBe(false);
  });
});
