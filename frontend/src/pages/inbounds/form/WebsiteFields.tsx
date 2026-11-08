import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Form, Input, Switch, Typography } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { HttpUtil, RandomUtil } from '@/utils';
import { WebsiteSchema, WebsiteStatusSchema } from '@/schemas/website';
import type { z } from 'zod';

export default function WebsiteFields({ inboundId }: { inboundId?: number }) {
  const { t } = useTranslation();
  const { control, getValues, setValue } = useFormContext();
  const site = useWatch({ control, name: 'streamSettings.website' }) as
    | z.infer<typeof WebsiteSchema>
    | undefined;
  const [status, setStatus] = useState<z.infer<typeof WebsiteStatusSchema> | null>(null);
  const [statusError, setStatusError] = useState('');
  useEffect(() => {
    if (!inboundId || !site?.enabled) return;
    let cancelled = false;
    const fetchStatus = async () => {
      try {
        const msg = await HttpUtil.get(
          `/panel/api/inbounds/${inboundId}/websiteStatus`,
          undefined,
          { silent: true },
        );
        const parsed = WebsiteStatusSchema.safeParse(msg?.obj);
        if (!cancelled) {
          if (msg?.success && parsed.success) {
            setStatus(parsed.data);
            setStatusError('');
          } else {
            setStatus(null);
            setStatusError(msg?.msg || t('somethingWentWrong'));
          }
        }
      } catch (error) {
        if (!cancelled) setStatusError(String(error));
      }
    };
    void fetchStatus();
    const timer = window.setInterval(() => {
      void fetchStatus();
    }, 5000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [inboundId, site?.enabled, t]);

  function enable(checked: boolean) {
    setValue('streamSettings.website', { ...WebsiteSchema.parse(site ?? {}), enabled: checked });
    if (!checked) return;
    const path = getValues('streamSettings.xhttpSettings.path');
    if (!path || path === '/') {
      const token = RandomUtil.randomUUID().replaceAll('-', '').slice(0, 16);
      setValue('streamSettings.xhttpSettings.path', `/connect-${token}/`);
    }
    const domain = getValues('streamSettings.tlsSettings.serverName') || getValues('shareAddr');
    if (!site?.domain && domain) setValue('streamSettings.website.domain', domain);
  }

  return (
    <>
      <FormField
        name={['streamSettings', 'website', 'enabled']}
        label={t('website.enabled')}
        valueProp="checked"
        onAfterChange={(value) => enable(value === true)}
      >
        <Switch />
      </FormField>
      {site?.enabled && (
        <>
          <Alert type="info" showIcon title={t('website.help')} style={{ marginBottom: 16 }} />
          <FormField
            name={['streamSettings', 'website', 'domain']}
            label={t('website.domain')}
            onAfterChange={(value) => {
              const domain = String(value).trim().toLowerCase();
              setValue('streamSettings.tlsSettings.serverName', domain);
              setValue('shareAddrStrategy', 'custom');
              setValue('shareAddr', domain);
            }}
          >
            <Input placeholder="www.example.com" />
          </FormField>
          <FormField
            name={['streamSettings', 'website', 'autoTLS']}
            label={t('website.autoTLS')}
            valueProp="checked"
            onAfterChange={(checked) => {
              if (checked) setValue('streamSettings.tlsSettings.certificates', []);
            }}
          >
            <Switch />
          </FormField>
          {site.autoTLS && (
            <>
              <Alert
                type="info"
                showIcon
                title={t('website.acmeHelp')}
                style={{ marginBottom: 16 }}
              />
              <Typography.Paragraph>
                <Typography.Link
                  href="https://letsencrypt.org/repository/"
                  target="_blank"
                  rel="noreferrer"
                >
                  {t('website.acmeTerms')}
                </Typography.Link>
              </Typography.Paragraph>
              <FormField name={['streamSettings', 'website', 'email']} label={t('website.email')}>
                <Input type="email" />
              </FormField>
            </>
          )}
          <FormField name={['streamSettings', 'website', 'title']} label={t('website.title')}>
            <Input maxLength={160} />
          </FormField>
          <FormField
            name={['streamSettings', 'website', 'description']}
            label={t('website.introduction')}
          >
            <Input.TextArea rows={3} maxLength={4000} />
          </FormField>
          {statusError && <Alert type="error" showIcon title={statusError} />}
          {status && (
            <Alert
              showIcon
              type={status.error ? 'error' : status.running ? 'success' : 'warning'}
              title={
                status.error ||
                t(
                  status.running
                    ? status.certificateReady
                      ? 'website.ready'
                      : 'website.awaitingCertificate'
                    : 'website.stopped',
                )
              }
            />
          )}
          {inboundId && site.domain && (
            <Form.Item label=" ">
              <Button href={`https://${site.domain}/`} target="_blank" rel="noreferrer">
                {t('website.open')}
              </Button>
            </Form.Item>
          )}
        </>
      )}
    </>
  );
}
