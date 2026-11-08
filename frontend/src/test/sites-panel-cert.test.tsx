import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import SitesPage from '@/pages/sites/SitesPage';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

vi.mock('@/layouts/AppSidebar', () => ({ default: () => null }));
vi.mock('@/api/queries/useSites', () => ({
  useSites: () => ({ data: [], isLoading: false, error: null }),
  useSiteTemplates: () => ({ data: [], isLoading: false, error: null }),
  templatePreviewURL: (id: string) => `/preview/${id}/`,
}));

async function openManualCertificateEditor() {
  renderWithProviders(
    <MemoryRouter>
      <SitesPage />
    </MemoryRouter>,
  );
  fireEvent.click(screen.getByRole('button', { name: /Create website/ }));
  fireEvent.click(
    await screen.findByRole('switch', { name: 'Obtain and renew certificates automatically' }),
  );
  return {
    cert: (await screen.findByLabelText('Certificate file')) as HTMLInputElement,
    key: screen.getByLabelText('Private key file') as HTMLInputElement,
  };
}

describe('site certificate shortcuts', () => {
  afterEach(() => vi.restoreAllMocks());

  it('fills the certificate paths from the panel settings and clears them again', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue({
      success: true,
      msg: '',
      obj: {
        webCertFile: '/root/cert/hk.example.com/fullchain.pem',
        webKeyFile: '/root/cert/hk.example.com/privkey.pem',
      },
    });
    const { cert, key } = await openManualCertificateEditor();
    fireEvent.click(screen.getByRole('button', { name: 'Set Cert from Panel' }));
    await waitFor(() => expect(cert.value).toBe('/root/cert/hk.example.com/fullchain.pem'));
    expect(key.value).toBe('/root/cert/hk.example.com/privkey.pem');

    fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
    await waitFor(() => expect(cert.value).toBe(''));
    expect(key.value).toBe('');
  });

  it('explains when the panel has no certificate configured', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue({ success: true, msg: '', obj: {} });
    const { cert } = await openManualCertificateEditor();
    fireEvent.click(screen.getByRole('button', { name: 'Set Cert from Panel' }));
    expect(
      await within(screen.getByRole('dialog')).findByText(
        'No certificate is configured for the panel. Set one under Settings first.',
      ),
    ).toBeTruthy();
    expect(cert.value).toBe('');
  });
});
