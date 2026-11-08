import { z } from 'zod';

export const WebsiteSchema = z.object({
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
