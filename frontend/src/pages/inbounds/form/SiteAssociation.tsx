import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Form, Select, Space, Typography } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';
import { useSites } from '@/api/queries/useSites';
import { FormField } from '@/components/form/rhf';
import { RandomUtil } from '@/utils';

type InboundCertificate = {
  certificateFile?: string;
  keyFile?: string;
  certificate?: string[];
  usage?: string;
};

// Mirrors inboundCertificateDomain: certificates a site wrote in earlier are not the inbound's own.
function hasOwnCertificate(
  certificates: InboundCertificate[] | undefined,
  previousSite: { certificateFile?: string; keyFile?: string } | undefined,
) {
  const usable = (certificates ?? []).filter(
    (c) => c.usage !== 'verify' && (c.certificateFile || c.certificate?.length),
  );
  if (usable.length === 0) return false;
  return !(
    previousSite?.certificateFile &&
    usable.length === 1 &&
    usable[0].certificateFile === previousSite.certificateFile &&
    usable[0].keyFile === previousSite.keyFile
  );
}

export default function SiteAssociation(_props: { inboundId?: number }) {
  const { t } = useTranslation();
  const selectorId = useId();
  const { control, setValue, getValues } = useFormContext();
  const { data: sites = [], isLoading, error } = useSites();
  const siteId = useWatch({ control, name: 'streamSettings.website.siteId' }) as number | undefined;
  const mode = useWatch({ control, name: 'streamSettings.website.mode' }) as string | undefined;
  const security = useWatch({ control, name: 'streamSettings.security' }) as string | undefined;
  const network = useWatch({ control, name: 'streamSettings.network' }) as string | undefined;
  const protocol = useWatch({ control, name: 'protocol' }) as string;
  let hint = 'independent';
  if (security === 'reality') hint = 'reality';
  else if (security === 'tls' && ['xhttp', 'ws', 'grpc', 'httpupgrade'].includes(network || ''))
    hint = 'path';
  else if (
    security === 'tls' &&
    ['tcp', 'raw'].includes(network || '') &&
    ['vless', 'trojan'].includes(protocol)
  )
    hint = 'fallback';

  const certificates = useWatch({ control, name: 'streamSettings.tlsSettings.certificates' }) as
    | InboundCertificate[]
    | undefined;
  const previousSite = useWatch({ control, name: 'streamSettings.website' }) as
    | { certificateFile?: string; keyFile?: string }
    | undefined;
  const usesOwnCertificate = hasOwnCertificate(certificates, previousSite);

  function choose(id: number | undefined) {
    const site = sites.find((item) => item.id === id);
    if (!site) {
      setValue('streamSettings.website', undefined);
      return;
    }
    setValue('streamSettings.website', { ...site.config, enabled: true, siteId: id, mode: 'auto' });
    if ((hint === 'path' || hint === 'fallback') && !usesOwnCertificate) {
      setValue('streamSettings.tlsSettings.serverName', site.config.domain);
      setValue(
        'streamSettings.tlsSettings.certificates',
        site.config.autoTLS
          ? []
          : [{ certificateFile: site.config.certificateFile, keyFile: site.config.keyFile }],
      );
      setValue('shareAddrStrategy', 'custom');
      setValue('shareAddr', site.config.domain);
    }
    if (hint === 'path') {
      const key =
        network === 'xhttp'
          ? 'xhttpSettings'
          : network === 'ws'
            ? 'wsSettings'
            : network === 'grpc'
              ? 'grpcSettings'
              : 'httpupgradeSettings';
      const field = network === 'grpc' ? 'serviceName' : 'path';
      const path = getValues(`streamSettings.${key}.${field}`);
      if (!path || path === '/')
        setValue(
          `streamSettings.${key}.${field}`,
          network === 'grpc'
            ? `connect-${RandomUtil.randomLowerAndNum(12)}`
            : `/connect-${RandomUtil.randomLowerAndNum(12)}/`,
        );
    }
  }

  return (
    <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Paragraph>{t('sites.associationHelp')}</Typography.Paragraph>
      {error && <Alert type="error" title={String(error)} />}
      <Form.Item label={t('menu.sites')} htmlFor={selectorId}>
        <Select
          id={selectorId}
          allowClear
          loading={isLoading}
          value={siteId}
          placeholder={t('sites.none')}
          options={sites.map((site) => ({
            value: site.id,
            label: `${site.name} · ${site.config.domain}`,
          }))}
          onChange={choose}
        />
      </Form.Item>
      {siteId && (
        <>
          <FormField name={['streamSettings', 'website', 'mode']} label={t('sites.mode')}>
            <Select
              options={['auto', 'path', 'fallback', 'reality', 'independent'].map((value) => ({
                value,
                label: t(`sites.modes.${value}`),
              }))}
            />
          </FormField>
          <Alert
            showIcon
            type="info"
            title={t(`sites.hints.${mode && mode !== 'auto' ? mode : hint}`)}
          />
          {hint === 'fallback' &&
            !usesOwnCertificate &&
            sites.find((site) => site.id === siteId)?.config.autoTLS && (
              <Alert showIcon type="warning" title={t('sites.fallbackCertificate')} />
            )}
        </>
      )}
    </Space>
  );
}
