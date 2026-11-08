import { describe, expect, it, vi } from 'vitest';
import { Form } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import SiteAssociation from '@/pages/inbounds/form/SiteAssociation';
import { renderWithProviders } from './test-utils';

vi.mock('@/api/queries/useSites', () => ({
  useSites: () => ({
    isLoading: false,
    error: null,
    data: [
      {
        id: 3,
        name: 'Journal',
        config: {
          enabled: true,
          domain: 'site.example.com',
          autoTLS: true,
          email: '',
          title: '',
          description: '',
          template: 'news',
          port: 443,
        },
      },
    ],
  }),
}));

function Harness({ certificates }: { certificates: Array<Record<string, string>> }) {
  const methods = useForm({
    defaultValues: {
      protocol: 'vless',
      shareAddr: 'hk2.example.com',
      shareAddrStrategy: 'custom',
      streamSettings: {
        network: 'xhttp',
        security: 'tls',
        xhttpSettings: { path: '/connect/' },
        tlsSettings: { serverName: 'hk2.example.com', certificates },
      },
    },
  });
  const values = useWatch({ control: methods.control });
  return (
    <FormProvider {...methods}>
      <Form>
        <SiteAssociation />
        <output data-testid="values">{JSON.stringify(values)}</output>
      </Form>
    </FormProvider>
  );
}

function chooseSite() {
  const select = screen.getByRole('combobox');
  fireEvent.mouseDown(select.closest('.ant-select') ?? select);
  const option = Array.from(document.querySelectorAll('.ant-select-item-option')).find((item) =>
    item.textContent?.startsWith('Journal'),
  );
  if (!option) throw new Error('site option not found');
  fireEvent.click(option);
}

const values = () => JSON.parse(screen.getByTestId('values').textContent!);

describe('site association', () => {
  // A site with its own auto certificate used to replace the inbound's certificate and SNI.
  it('keeps the inbound certificate, SNI and share address when the inbound has its own certificate', async () => {
    const own = [{ certificateFile: '/etc/ssl/hk2.pem', keyFile: '/etc/ssl/hk2.key' }];
    renderWithProviders(<Harness certificates={own} />);
    chooseSite();
    await waitFor(() => expect(values().streamSettings.website?.siteId).toBe(3));
    expect(values()).toMatchObject({
      shareAddr: 'hk2.example.com',
      streamSettings: { tlsSettings: { serverName: 'hk2.example.com', certificates: own } },
    });
  });

  it('uses the site certificate when the inbound has none', async () => {
    renderWithProviders(<Harness certificates={[]} />);
    chooseSite();
    await waitFor(() =>
      expect(values()).toMatchObject({
        shareAddr: 'site.example.com',
        streamSettings: { tlsSettings: { serverName: 'site.example.com', certificates: [] } },
      }),
    );
  });
});
