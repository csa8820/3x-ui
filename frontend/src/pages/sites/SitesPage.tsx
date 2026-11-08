import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  Empty,
  Form,
  Input,
  InputNumber,
  Layout,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { useSites, useSiteTemplates, templatePreviewURL } from '@/api/queries/useSites';
import { HttpUtil } from '@/utils';
import { WebsiteSchema, type SiteRecord } from '@/schemas/website';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import type { z } from 'zod';

type Values = z.infer<typeof WebsiteSchema> & { name: string };

export default function SitesPage() {
  const { t } = useTranslation();
  const { antdThemeConfig, isDark, isUltra } = useTheme();
  const sites = useSites();
  const templates = useSiteTemplates();
  const [form] = Form.useForm<Values>();
  const [editing, setEditing] = useState<number | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [galleryOpen, setGalleryOpen] = useState(false);
  const [pending, setPending] = useState('');
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const upload = useRef<HTMLInputElement>(null);
  const autoTLS = Form.useWatch('autoTLS', form);
  const selectedTemplate = Form.useWatch('template', form);
  const rows = sites.data ?? [];
  const gallery = (templates.data ?? []).filter((item) =>
    `${item.name} ${item.category} ${item.source}`.toLowerCase().includes(search.toLowerCase()),
  );

  async function applyPanelCertificate() {
    setError('');
    setPending('panelCert');
    try {
      const response = await HttpUtil.post('/panel/api/setting/all', undefined, { silent: true });
      const panel = response.obj as { webCertFile?: string; webKeyFile?: string } | undefined;
      if (!response.success || !panel?.webCertFile || !panel.webKeyFile) {
        setError(t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      form.setFieldsValue({ certificateFile: panel.webCertFile, keyFile: panel.webKeyFile });
    } catch (e) {
      if (e instanceof Error) setError(e.message);
    } finally {
      setPending('');
    }
  }

  function edit(site?: SiteRecord, template = 'studio') {
    setError('');
    setEditing(site?.id ?? null);
    form.resetFields();
    form.setFieldsValue(
      site
        ? { ...site.config, name: site.name }
        : {
            ...WebsiteSchema.parse({}),
            name: '',
            template,
            autoTLS: true,
            port: 443,
            standalone: false,
          },
    );
    setEditorOpen(true);
  }
  async function save() {
    try {
      const values = await form.validateFields();
      const { name, ...config } = values;
      const parsed = WebsiteSchema.parse({ ...config, enabled: true });
      setPending('save');
      setError('');
      const response = await HttpUtil.post(
        '/panel/api/sites/save',
        { id: editing ?? 0, name, config: JSON.stringify(parsed) },
        { silent: true },
      );
      if (!response.success) {
        setError(response.msg);
        return;
      }
      setEditorOpen(false);
      await sites.refetch();
    } catch (e) {
      if (e instanceof Error) setError(e.message);
    } finally {
      setPending('');
    }
  }
  async function install(id: string) {
    setPending(id);
    setError('');
    try {
      const response = await HttpUtil.post(
        '/panel/api/sites/templates/install',
        { id },
        { silent: true },
      );
      if (!response.success) setError(response.msg);
      else await templates.refetch();
    } finally {
      setPending('');
    }
  }
  async function importZIP(file: File) {
    if (file.size > 25 * 1024 * 1024) {
      setError(t('sites.zipLimit'));
      return;
    }
    setPending('import');
    setError('');
    try {
      const data = new FormData();
      data.append('file', file);
      const response = await HttpUtil.post('/panel/api/sites/templates/import', data, {
        silent: true,
      });
      if (!response.success) setError(response.msg);
      else {
        await templates.refetch();
        setGalleryOpen(true);
      }
    } finally {
      setPending('');
      if (upload.current) upload.current.value = '';
    }
  }

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout
        className={`sites-page nodes-page${isDark ? ' is-dark' : ''}${isUltra ? ' is-ultra' : ''}`}
      >
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Space orientation="vertical" size="large" style={{ width: '100%' }}>
              <div>
                <Typography.Title level={2}>{t('menu.sites')}</Typography.Title>
                <Typography.Paragraph type="secondary">{t('sites.subtitle')}</Typography.Paragraph>
              </div>
              <Space wrap>
                <Button type="primary" onClick={() => edit()}>
                  {t('sites.create')}
                </Button>
                <Button onClick={() => setGalleryOpen(true)}>{t('sites.gallery')}</Button>
                <Button loading={pending === 'import'} onClick={() => upload.current?.click()}>
                  {t('sites.importZIP')}
                </Button>
              </Space>
              <input
                ref={upload}
                type="file"
                accept=".zip,application/zip"
                hidden
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  if (file) void importZIP(file);
                }}
              />
              {(error || sites.error || templates.error) && (
                <Alert
                  showIcon
                  type="error"
                  title={error || String(sites.error || templates.error)}
                />
              )}
              <Spin aria-live="off" spinning={sites.isLoading}>
                {rows.length === 0 && !sites.isLoading ? (
                  <Empty description={t('sites.empty')} />
                ) : (
                  <Row gutter={[16, 16]}>
                    {rows.map((site) => (
                      <Col xs={24} md={12} xl={8} key={site.id}>
                        <Card
                          title={site.name}
                          extra={
                            <Tag>
                              {site.config.standalone ? t('sites.standalone') : t('sites.onDemand')}
                            </Tag>
                          }
                        >
                          <Typography.Paragraph>
                            {site.config.domain} · {site.config.port ?? 443}
                          </Typography.Paragraph>
                          {site.status && (
                            <Tag
                              color={
                                site.status.error
                                  ? 'error'
                                  : site.status.running
                                    ? 'success'
                                    : 'default'
                              }
                            >
                              {site.status.error ||
                                t(
                                  site.status.running
                                    ? site.status.certificateReady
                                      ? 'website.ready'
                                      : 'website.awaitingCertificate'
                                    : 'website.stopped',
                                )}
                            </Tag>
                          )}
                          <Typography.Paragraph type="secondary">
                            {t('sites.references', { count: site.inboundIds?.length ?? 0 })}
                          </Typography.Paragraph>
                          <Typography.Paragraph type="secondary">
                            {(templates.data ?? []).find((item) => item.id === site.config.template)
                              ?.name ??
                              site.config.template ??
                              'Studio'}
                          </Typography.Paragraph>
                          <Space>
                            <Button onClick={() => edit(site)}>{t('edit')}</Button>
                            <Popconfirm
                              title={t('sites.deleteConfirm')}
                              onConfirm={async () => {
                                const response = await HttpUtil.post(
                                  `/panel/api/sites/del/${site.id}`,
                                  undefined,
                                  { silent: true },
                                );
                                if (!response.success) setError(response.msg);
                                else await sites.refetch();
                              }}
                            >
                              <Button danger>{t('delete')}</Button>
                            </Popconfirm>
                          </Space>
                        </Card>
                      </Col>
                    ))}
                  </Row>
                )}
              </Spin>
              <Alert type="info" showIcon title={t('sites.bindHelp')} />
            </Space>
          </Layout.Content>
        </Layout>
      </Layout>
      <Modal
        title={t('sites.editor')}
        open={editorOpen}
        onCancel={() => setEditorOpen(false)}
        onOk={() => void save()}
        confirmLoading={pending === 'save'}
        width={760}
        centered
        styles={{ body: { maxHeight: '65vh', overflowY: 'auto', overflowX: 'hidden' } }}
        forceRender
      >
        <Form form={form} layout="vertical">
          {error && <Alert type="error" title={error} />}
          <Form.Item name="name" label={t('sites.name')} rules={[{ required: true }]}>
            <Input maxLength={80} />
          </Form.Item>
          <Row gutter={16}>
            <Col span={16}>
              <Form.Item
                name="domain"
                label={t('website.domain')}
                rules={[
                  { required: true },
                  { pattern: /^[a-z0-9.-]+\.[a-z]{2,}$/i, message: t('website.invalidDomain') },
                ]}
              >
                <Input placeholder="www.example.com" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="port" label={t('pages.inbounds.port')} rules={[{ required: true }]}>
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="template" label={t('sites.template')} rules={[{ required: true }]}>
            <Select
              options={(templates.data ?? [])
                .filter((item) => item.installed)
                .map((item) => ({ value: item.id, label: item.name }))}
            />
          </Form.Item>
          {selectedTemplate && (
            <div
              style={{
                position: 'relative',
                width: '100%',
                height: 240,
                overflow: 'hidden',
                borderRadius: 8,
              }}
            >
              <iframe
                title={t('sites.preview')}
                src={templatePreviewURL(selectedTemplate)}
                sandbox=""
                style={{
                  position: 'absolute',
                  width: '200%',
                  height: '200%',
                  border: 0,
                  transform: 'scale(0.5)',
                  transformOrigin: 'top left',
                }}
              />
            </div>
          )}
          <Form.Item name="title" label={t('website.title')}>
            <Input maxLength={160} />
          </Form.Item>
          <Form.Item
            name="description"
            label={t('website.introduction')}
            extra={t('sites.importedContent')}
          >
            <Input.TextArea rows={3} maxLength={4000} />
          </Form.Item>
          <Form.Item
            name="standalone"
            label={t('sites.standalone')}
            valuePropName="checked"
            extra={t('sites.standaloneHelp')}
          >
            <Switch />
          </Form.Item>
          <Form.Item name="autoTLS" label={t('website.autoTLS')} valuePropName="checked">
            <Switch />
          </Form.Item>
          {autoTLS ? (
            <>
              <Alert showIcon type="info" title={t('website.acmeHelp')} />
              <Typography.Link
                href="https://letsencrypt.org/repository/"
                target="_blank"
                rel="noreferrer"
              >
                {t('website.acmeTerms')}
              </Typography.Link>
              <Form.Item name="email" label={t('website.email')}>
                <Input type="email" />
              </Form.Item>
            </>
          ) : (
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name="certificateFile"
                  label={t('sites.certificateFile')}
                  rules={[{ required: true }]}
                >
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="keyFile" label={t('sites.keyFile')} rules={[{ required: true }]}>
                  <Input />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Space>
                  <Button
                    type="primary"
                    loading={pending === 'panelCert'}
                    onClick={() => void applyPanelCertificate()}
                  >
                    {t('pages.inbounds.setDefaultCert')}
                  </Button>
                  <Button
                    danger
                    onClick={() => form.setFieldsValue({ certificateFile: '', keyFile: '' })}
                  >
                    {t('clear')}
                  </Button>
                </Space>
              </Col>
            </Row>
          )}
        </Form>
      </Modal>
      <Modal
        title={t('sites.gallery')}
        open={galleryOpen}
        onCancel={() => setGalleryOpen(false)}
        footer={null}
        width={1100}
        centered
        styles={{ body: { maxHeight: '75vh', overflowY: 'auto', overflowX: 'hidden' } }}
      >
        <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
          <Input.Search
            aria-label={t('sites.search')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('sites.search')}
          />
          <Typography.Paragraph type="secondary">
            {t('sites.catalogHelp')}{' '}
            <Typography.Link
              href="https://github.com/StartBootstrap"
              target="_blank"
              rel="noreferrer"
            >
              Start Bootstrap
            </Typography.Link>
          </Typography.Paragraph>
          {error && <Alert type="error" title={error} />}
          <Row gutter={[16, 16]}>
            {gallery.map((item) => (
              <Col key={item.id} xs={24} sm={12} lg={8}>
                <Card title={item.name} size="small">
                  {item.installed ? (
                    <div
                      style={{
                        position: 'relative',
                        width: '100%',
                        height: 180,
                        overflow: 'hidden',
                      }}
                    >
                      <iframe
                        title={`${t('sites.preview')} ${item.name}`}
                        src={templatePreviewURL(item.id)}
                        sandbox=""
                        loading="lazy"
                        style={{
                          position: 'absolute',
                          width: '360%',
                          height: '360%',
                          border: 0,
                          transform: 'scale(0.2777778)',
                          transformOrigin: 'top left',
                          pointerEvents: 'none',
                        }}
                      />
                    </div>
                  ) : (
                    <div
                      style={{
                        height: 150,
                        display: 'grid',
                        placeItems: 'center',
                        background: '#f0f2ed',
                        color: '#425449',
                        fontSize: 22,
                      }}
                    >
                      {item.category}
                    </div>
                  )}
                  <Space wrap style={{ margin: '12px 0' }}>
                    <Tag>{item.category}</Tag>
                    <Tag>{item.source}</Tag>
                    {item.license && <Tag>{item.license}</Tag>}
                  </Space>
                  <div>
                    {item.installed ? (
                      <Button
                        type="primary"
                        onClick={() => {
                          if (editorOpen) form.setFieldValue('template', item.id);
                          else edit(undefined, item.id);
                          setGalleryOpen(false);
                        }}
                      >
                        {t('sites.useTemplate')}
                      </Button>
                    ) : (
                      <Space>
                        <Button
                          loading={pending === item.id}
                          disabled={!!pending}
                          onClick={() => void install(item.id)}
                        >
                          {t('sites.install')}
                        </Button>
                        <Button
                          href={`https://startbootstrap.github.io/${item.repository}/`}
                          target="_blank"
                          rel="noreferrer"
                        >
                          {t('sites.preview')}
                        </Button>
                      </Space>
                    )}
                  </div>
                </Card>
              </Col>
            ))}
          </Row>
        </Space>
      </Modal>
    </ConfigProvider>
  );
}
