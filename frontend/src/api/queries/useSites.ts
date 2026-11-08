import { useQuery } from '@tanstack/react-query';
import { HttpUtil } from '@/utils';
import { SiteListSchema, TemplateListSchema } from '@/schemas/website';

export function useSites() {
  return useQuery({
    queryKey: ['sites'],
    refetchInterval: 5000,
    queryFn: async () => {
      const response = await HttpUtil.get('/panel/api/sites/list', undefined, { silent: true });
      if (!response.success) throw new Error(response.msg);
      return SiteListSchema.parse(response.obj);
    },
  });
}

export function useSiteTemplates() {
  return useQuery({
    queryKey: ['site-templates'],
    queryFn: async () => {
      const response = await HttpUtil.get('/panel/api/sites/templates', undefined, {
        silent: true,
      });
      if (!response.success) throw new Error(response.msg);
      return TemplateListSchema.parse(response.obj);
    },
  });
}

export function templatePreviewURL(id: string) {
  const base = (window.X_UI_BASE_PATH || '/').replace(/\/$/, '');
  return `${base}/panel/api/sites/templates/preview/${encodeURIComponent(id)}/`;
}
