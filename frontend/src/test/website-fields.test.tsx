import { describe, expect, it, vi } from 'vitest';
import { Form } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import WebsiteFields from '@/pages/inbounds/form/WebsiteFields';
import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

function Harness() {
  const methods = useForm({
    defaultValues: {
      shareAddr: '',
      shareAddrStrategy: 'node',
      streamSettings: {
        xhttpSettings: { path: '/' },
        tlsSettings: { serverName: '', certificates: [{ certificateFile: '/existing.pem' }] },
        website: { enabled: false, domain: '', autoTLS: false },
      },
    },
  });
  const values = useWatch({ control: methods.control });
  return (
    <FormProvider {...methods}>
      <Form>
        <WebsiteFields />
        <output data-testid="values">{JSON.stringify(values)}</output>
      </Form>
    </FormProvider>
  );
}

describe('website controls', () => {
  it('enables the site, creates a separate path and synchronizes the advertised TLS domain', async () => {
    renderWithProviders(<Harness />);
    fireEvent.click(screen.getByRole('switch', { name: 'Built-in website' }));
    await waitFor(() =>
      expect(
        JSON.parse(screen.getByTestId('values').textContent!).streamSettings.xhttpSettings.path,
      ).toMatch(/^\/connect-[a-f0-9]+\/$/),
    );
    fireEvent.change(screen.getByLabelText('Website domain'), {
      target: { value: 'site.example.com' },
    });
    fireEvent.click(
      screen.getByRole('switch', { name: 'Obtain and renew certificates automatically' }),
    );
    await waitFor(() =>
      expect(JSON.parse(screen.getByTestId('values').textContent!)).toMatchObject({
        shareAddr: 'site.example.com',
        shareAddrStrategy: 'custom',
        streamSettings: {
          tlsSettings: { serverName: 'site.example.com', certificates: [] },
          website: { enabled: true, domain: 'site.example.com', autoTLS: true },
        },
      }),
    );
  });

  it('creates a usable client and public TLS endpoint from the one-click preset', async () => {
    const onSaved = vi.fn();
    const post = vi
      .spyOn(HttpUtil, 'post')
      .mockResolvedValue({ success: true, msg: '', obj: { id: 17 } });
    renderWithProviders(
      <InboundFormModal
        open
        mode="add"
        websitePreset
        dbInbound={null}
        dbInbounds={[]}
        availableNodes={[]}
        onClose={() => {}}
        onSaved={onSaved}
      />,
    );
    fireEvent.change(await screen.findByLabelText('Website domain'), {
      target: { value: 'site.example.com' },
    });
    const save = document.querySelector<HTMLButtonElement>('.ant-modal-footer .ant-btn-primary');
    if (!save) throw new Error('Save button is missing');
    fireEvent.click(save);
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    const call = post.mock.calls.find(([url]) => url === '/panel/api/inbounds/add');
    expect(call).toBeDefined();
    const payload = call![1] as Record<string, unknown>;
    expect(payload).toMatchObject({ port: 443, protocol: 'vless', shareAddr: 'site.example.com' });
    expect(JSON.parse(String(payload.settings)).clients).toHaveLength(1);
    expect(JSON.parse(String(payload.streamSettings))).toMatchObject({
      network: 'xhttp',
      security: 'tls',
      website: { enabled: true, autoTLS: true },
    });
    post.mockRestore();
  });
});
