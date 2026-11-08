import { z } from 'zod';

export const WebsiteSchema = z.object({
  siteId: z.number().int().min(0).optional(),
  mode: z.enum(['auto', 'path', 'fallback', 'reality', 'independent']).optional(),
  template: z.string().optional(),
  standalone: z.boolean().optional(),
  listen: z.string().optional(),
  port: z.number().int().min(1).max(65535).optional(),
  certificateFile: z.string().optional(),
  keyFile: z.string().optional(),
  enabled: z.boolean().default(false),
  domain: z.string().default(''),
  autoTLS: z.boolean().default(false),
  email: z.string().default(''),
  title: z.string().max(160).default(''),
  description: z.string().max(4000).default(''),
});

export const WebsiteStatusSchema = z.object({
  running: z.boolean(),
  certificateReady: z.boolean(),
  error: z.string(),
});

export const SiteRecordSchema = z.object({
  id: z.number(),
  name: z.string(),
  status: WebsiteStatusSchema.optional(),
  inboundIds: z.array(z.number()).optional(),
  config: z.preprocess((value) => {
    if (typeof value !== 'string') return value;
    try {
      return JSON.parse(value);
    } catch {
      return undefined;
    }
  }, WebsiteSchema),
});
export const SiteListSchema = z.array(SiteRecordSchema);
export type SiteRecord = z.infer<typeof SiteRecordSchema>;
export const TemplateInfoSchema = z.object({
  id: z.string(),
  name: z.string(),
  category: z.string(),
  source: z.string(),
  license: z.string(),
  repository: z.string().optional(),
  installed: z.boolean(),
});
export const TemplateListSchema = z.array(TemplateInfoSchema);
export type TemplateInfo = z.infer<typeof TemplateInfoSchema>;
