export type FieldKind = 'text' | 'number' | 'boolean' | 'array' | 'select';
export type SectionKey = 'proxies' | 'visitors' | 'custom';
export type FieldState = string | boolean;
export type SerializableFieldValue =
  | string
  | number
  | boolean
  | Array<string | number | boolean>;

export interface FieldOption {
  label: string;
  value: string;
}

export interface FieldSchema {
  key: string;
  label: string;
  kind: FieldKind;
  required?: boolean;
  placeholder?: string;
  help?: string;
  options?: FieldOption[];
  defaultValue?: string | number | boolean;
  showWhen?: (values: Record<string, FieldState>) => boolean;
}

export interface TemplatePreset {
  id: string;
  sectionKey: SectionKey;
  label: string;
  description: string;
  hiddenEntries?: Record<string, SerializableFieldValue>;
  fields: FieldSchema[];
}

export interface SectionGroup {
  key: SectionKey;
  label: string;
  templates: TemplatePreset[];
}

const pluginOptions: FieldOption[] = [
  { label: 'http_proxy', value: 'http_proxy' },
  { label: 'socks5', value: 'socks5' },
  { label: 'static_file', value: 'static_file' },
  { label: 'unix_domain_socket', value: 'unix_domain_socket' },
  { label: 'http2https', value: 'http2https' },
  { label: 'https2http', value: 'https2http' },
  { label: 'https2https', value: 'https2https' },
  { label: 'tls2raw', value: 'tls2raw' },
  { label: 'virtual_net', value: 'virtual_net' },
];

const healthCheckOptions: FieldOption[] = [
  { label: 'tcp', value: 'tcp' },
  { label: 'http', value: 'http' },
];

const tcpMuxMultiplexerOptions: FieldOption[] = [
  { label: 'httpconnect', value: 'httpconnect' },
];

const xtcpProtocolOptions: FieldOption[] = [
  { label: 'quic', value: 'quic' },
  { label: 'kcp', value: 'kcp' },
  { label: 'websocket', value: 'websocket' },
  { label: 'wss', value: 'wss' },
];

const isPluginType = (...types: string[]) => (values: Record<string, FieldState>) =>
  types.includes(String(values['plugin.type'] || ''));

const isHealthCheckType = (...types: string[]) => (values: Record<string, FieldState>) =>
  types.includes(String(values['healthCheck.type'] || ''));

const textField = (
  key: string,
  label: string,
  options?: Pick<FieldSchema, 'required' | 'placeholder' | 'help' | 'defaultValue' | 'showWhen'>,
): FieldSchema => ({
  key,
  label,
  kind: 'text',
  ...options,
});

const numberField = (
  key: string,
  label: string,
  options?: Pick<FieldSchema, 'required' | 'placeholder' | 'help' | 'defaultValue' | 'showWhen'>,
): FieldSchema => ({
  key,
  label,
  kind: 'number',
  ...options,
});

const booleanField = (
  key: string,
  label: string,
  options?: Pick<FieldSchema, 'help' | 'defaultValue' | 'showWhen'>,
): FieldSchema => ({
  key,
  label,
  kind: 'boolean',
  ...options,
});

const arrayField = (
  key: string,
  label: string,
  options?: Pick<FieldSchema, 'required' | 'placeholder' | 'help' | 'defaultValue' | 'showWhen'>,
): FieldSchema => ({
  key,
  label,
  kind: 'array',
  ...options,
});

const selectField = (
  key: string,
  label: string,
  options: FieldOption[],
  extra?: Pick<FieldSchema, 'required' | 'help' | 'defaultValue' | 'showWhen'>,
): FieldSchema => ({
  key,
  label,
  kind: 'select',
  options,
  ...extra,
});

const proxyIdentityFields: FieldSchema[] = [
  textField('name', '名称', {
    required: true,
    placeholder: 'device_b_ssh',
  }),
  textField('localIP', '本地 IP', {
    defaultValue: '127.0.0.1',
    placeholder: '127.0.0.1',
  }),
  numberField('localPort', '本地端口', {
    required: true,
    placeholder: '22',
  }),
];

const proxyTransportFields: FieldSchema[] = [
  booleanField('transport.useEncryption', 'transport.useEncryption'),
  booleanField('transport.useCompression', 'transport.useCompression'),
  textField('transport.bandwidthLimit', 'transport.bandwidthLimit', {
    placeholder: '1MB',
  }),
  selectField(
    'transport.bandwidthLimitMode',
    'transport.bandwidthLimitMode',
    [
      { label: 'client', value: 'client' },
      { label: 'server', value: 'server' },
    ],
  ),
  selectField(
    'transport.proxyProtocolVersion',
    'transport.proxyProtocolVersion',
    [
      { label: 'v1', value: 'v1' },
      { label: 'v2', value: 'v2' },
    ],
  ),
  numberField('transport.poolCount', 'transport.poolCount', {
    placeholder: '5',
  }),
];

const proxyLoadBalancerFields: FieldSchema[] = [
  textField('loadBalancer.group', 'loadBalancer.group', {
    placeholder: 'web-group',
  }),
  textField('loadBalancer.groupKey', 'loadBalancer.groupKey', {
    placeholder: 'group-secret',
  }),
];

const proxyHealthCheckFields: FieldSchema[] = [
  selectField('healthCheck.type', 'healthCheck.type', healthCheckOptions),
  numberField('healthCheck.timeoutSeconds', 'healthCheck.timeoutSeconds', {
    placeholder: '3',
  }),
  numberField('healthCheck.maxFailed', 'healthCheck.maxFailed', {
    placeholder: '1',
  }),
  numberField('healthCheck.intervalSeconds', 'healthCheck.intervalSeconds', {
    placeholder: '10',
  }),
  textField('healthCheck.path', 'healthCheck.path', {
    placeholder: '/healthz',
    showWhen: isHealthCheckType('http'),
  }),
];

const proxyPluginFields: FieldSchema[] = [
  selectField('plugin.type', 'plugin.type', pluginOptions),
  textField('plugin.httpUser', 'plugin.httpUser', {
    placeholder: 'demo',
    showWhen: isPluginType('http_proxy', 'static_file'),
  }),
  textField('plugin.httpPassword', 'plugin.httpPassword', {
    placeholder: 'password',
    showWhen: isPluginType('http_proxy', 'static_file'),
  }),
  textField('plugin.username', 'plugin.username', {
    placeholder: 'demo',
    showWhen: isPluginType('socks5'),
  }),
  textField('plugin.password', 'plugin.password', {
    placeholder: 'password',
    showWhen: isPluginType('socks5'),
  }),
  textField('plugin.localPath', 'plugin.localPath', {
    placeholder: '/var/www/html',
    showWhen: isPluginType('static_file'),
  }),
  textField('plugin.stripPrefix', 'plugin.stripPrefix', {
    placeholder: '/static',
    showWhen: isPluginType('static_file'),
  }),
  textField('plugin.unixPath', 'plugin.unixPath', {
    placeholder: '/var/run/docker.sock',
    showWhen: isPluginType('unix_domain_socket'),
  }),
  textField('plugin.localAddr', 'plugin.localAddr', {
    placeholder: '127.0.0.1:8080',
    showWhen: isPluginType('http2https', 'https2http', 'https2https', 'tls2raw'),
  }),
  textField('plugin.hostHeaderRewrite', 'plugin.hostHeaderRewrite', {
    placeholder: 'example.com',
    showWhen: isPluginType('http2https', 'https2http', 'https2https'),
  }),
  textField('plugin.crtPath', 'plugin.crtPath', {
    placeholder: '/path/to/fullchain.pem',
    showWhen: isPluginType('http2https', 'https2http', 'https2https', 'tls2raw'),
  }),
  textField('plugin.keyPath', 'plugin.keyPath', {
    placeholder: '/path/to/privkey.pem',
    showWhen: isPluginType('http2https', 'https2http', 'https2https', 'tls2raw'),
  }),
  booleanField('plugin.enableHTTP2', 'plugin.enableHTTP2', {
    showWhen: isPluginType('https2https'),
  }),
  textField('plugin.network', 'plugin.network', {
    placeholder: '192.168.111.0/24',
    showWhen: isPluginType('virtual_net'),
  }),
];

const proxyOptionalFields: FieldSchema[] = [
  ...proxyTransportFields,
  ...proxyLoadBalancerFields,
  ...proxyHealthCheckFields,
  ...proxyPluginFields,
];

const visitorBaseFields: FieldSchema[] = [
  textField('name', '名称', {
    required: true,
    placeholder: 'visit_a_ssh',
  }),
  textField('serverName', 'serverName', {
    required: true,
    placeholder: 'device_a_ssh',
  }),
  textField('secretKey', 'secretKey', {
    required: true,
    placeholder: 'shared-secret',
  }),
  textField('bindAddr', 'bindAddr', {
    defaultValue: '127.0.0.1',
    placeholder: '127.0.0.1',
  }),
  numberField('bindPort', 'bindPort', {
    required: true,
    placeholder: '6000',
  }),
];

const proxyTemplates: TemplatePreset[] = [
  {
    id: 'proxy-tcp',
    sectionKey: 'proxies',
    label: 'TCP 端口映射',
    description: '官方 tcp 代理模板，适合 SSH、数据库、RDP 等常规 TCP 服务。',
    hiddenEntries: {
      type: 'tcp',
    },
    fields: [
      ...proxyIdentityFields,
      numberField('remotePort', 'remotePort', {
        required: true,
        placeholder: '6000',
      }),
      ...proxyOptionalFields,
    ],
  },
  {
    id: 'proxy-udp',
    sectionKey: 'proxies',
    label: 'UDP 端口映射',
    description: '官方 udp 代理模板，适合 DNS、游戏服、语音等 UDP 服务。',
    hiddenEntries: {
      type: 'udp',
    },
    fields: [
      ...proxyIdentityFields,
      numberField('remotePort', 'remotePort', {
        required: true,
        placeholder: '6001',
      }),
      ...proxyOptionalFields,
    ],
  },
  {
    id: 'proxy-http',
    sectionKey: 'proxies',
    label: 'HTTP 网站代理',
    description: '官方 http 代理模板，支持域名、路径、鉴权和健康检查。',
    hiddenEntries: {
      type: 'http',
    },
    fields: [
      ...proxyIdentityFields,
      arrayField('customDomains', 'customDomains', {
        placeholder: 'a.example.com, b.example.com',
      }),
      textField('subdomain', 'subdomain', {
        placeholder: 'demo-web',
      }),
      arrayField('locations', 'locations', {
        placeholder: '/,/admin',
      }),
      textField('hostHeaderRewrite', 'hostHeaderRewrite', {
        placeholder: 'internal.service.local',
      }),
      textField('httpUser', 'httpUser', {
        placeholder: 'demo',
      }),
      textField('httpPassword', 'httpPassword', {
        placeholder: 'password',
      }),
      ...proxyOptionalFields,
    ],
  },
  {
    id: 'proxy-https',
    sectionKey: 'proxies',
    label: 'HTTPS 网站代理',
    description: '官方 https 代理模板，适合 HTTPS 站点的域名映射。',
    hiddenEntries: {
      type: 'https',
    },
    fields: [
      ...proxyIdentityFields,
      arrayField('customDomains', 'customDomains', {
        placeholder: 'secure.example.com',
      }),
      textField('subdomain', 'subdomain', {
        placeholder: 'secure-app',
      }),
      textField('hostHeaderRewrite', 'hostHeaderRewrite', {
        placeholder: 'origin.example.internal',
      }),
      ...proxyOptionalFields,
    ],
  },
  {
    id: 'proxy-tcpmux',
    sectionKey: 'proxies',
    label: 'TCPMUX 复用代理',
    description: '官方 tcpmux 代理模板，适合通过同一端口复用多个服务。',
    hiddenEntries: {
      type: 'tcpmux',
    },
    fields: [
      ...proxyIdentityFields,
      selectField('multiplexer', 'multiplexer', tcpMuxMultiplexerOptions, {
        defaultValue: 'httpconnect',
      }),
      arrayField('customDomains', 'customDomains', {
        placeholder: 'mux.example.com',
      }),
      textField('subdomain', 'subdomain', {
        placeholder: 'mux-service',
      }),
      textField('httpUser', 'httpUser', {
        placeholder: 'demo',
      }),
      textField('httpPassword', 'httpPassword', {
        placeholder: 'password',
      }),
      textField('routeByHTTPUser', 'routeByHTTPUser', {
        placeholder: 'alice',
      }),
      ...proxyOptionalFields,
    ],
  },
  {
    id: 'proxy-stcp',
    sectionKey: 'proxies',
    label: 'STCP 私有代理',
    description: '官方 stcp 代理模板，适合只允许指定访问器连接的私有 TCP 服务。',
    hiddenEntries: {
      type: 'stcp',
    },
    fields: [
      ...proxyIdentityFields,
      textField('secretKey', 'secretKey', {
        required: true,
        placeholder: 'shared-secret',
      }),
      arrayField('allowUsers', 'allowUsers', {
        placeholder: 'alice, bob',
      }),
      ...proxyOptionalFields,
    ],
  },
  {
    id: 'proxy-xtcp',
    sectionKey: 'proxies',
    label: 'XTCP 点对点代理',
    description: '官方 xtcp 代理模板，适合 P2P 内网穿透。',
    hiddenEntries: {
      type: 'xtcp',
    },
    fields: [
      ...proxyIdentityFields,
      textField('secretKey', 'secretKey', {
        required: true,
        placeholder: 'shared-secret',
      }),
      arrayField('allowUsers', 'allowUsers', {
        placeholder: 'alice, bob',
      }),
      ...proxyOptionalFields,
    ],
  },
  {
    id: 'proxy-sudp',
    sectionKey: 'proxies',
    label: 'SUDP 私有 UDP',
    description: '官方 sudp 代理模板，适合私有 UDP 服务。',
    hiddenEntries: {
      type: 'sudp',
    },
    fields: [
      ...proxyIdentityFields,
      textField('secretKey', 'secretKey', {
        required: true,
        placeholder: 'shared-secret',
      }),
      arrayField('allowUsers', 'allowUsers', {
        placeholder: 'alice, bob',
      }),
      ...proxyOptionalFields,
    ],
  },
];

const visitorTemplates: TemplatePreset[] = [
  {
    id: 'visitor-stcp',
    sectionKey: 'visitors',
    label: 'STCP Visitor',
    description: '官方 stcp visitor 模板，用于访问 stcp 代理。',
    hiddenEntries: {
      type: 'stcp',
    },
    fields: [...visitorBaseFields],
  },
  {
    id: 'visitor-xtcp',
    sectionKey: 'visitors',
    label: 'XTCP Visitor',
    description: '官方 xtcp visitor 模板，支持协议选择、保持隧道和回落目标。',
    hiddenEntries: {
      type: 'xtcp',
    },
    fields: [
      ...visitorBaseFields,
      selectField('protocol', 'protocol', xtcpProtocolOptions),
      booleanField('keepTunnelOpen', 'keepTunnelOpen'),
      textField('fallbackTo', 'fallbackTo', {
        placeholder: '127.0.0.1:22',
      }),
      numberField('fallbackTimeoutMs', 'fallbackTimeoutMs', {
        placeholder: '200',
      }),
    ],
  },
  {
    id: 'visitor-sudp',
    sectionKey: 'visitors',
    label: 'SUDP Visitor',
    description: '官方 sudp visitor 模板，用于访问 sudp 代理。',
    hiddenEntries: {
      type: 'sudp',
    },
    fields: [...visitorBaseFields],
  },
];

const customTemplates: TemplatePreset[] = [
  {
    id: 'custom-array',
    sectionKey: 'custom',
    label: '自定义数组表',
    description: '手工填写段落名，再通过额外属性补齐任何未内置的字段。',
    fields: [],
  },
];

export const sectionGroups: SectionGroup[] = [
  {
    key: 'proxies',
    label: '代理段落',
    templates: proxyTemplates,
  },
  {
    key: 'visitors',
    label: '访问器段落',
    templates: visitorTemplates,
  },
  {
    key: 'custom',
    label: '自定义段落',
    templates: customTemplates,
  },
];

export const sectionGroupMap = Object.fromEntries(
  sectionGroups.map((group) => [group.key, group]),
) as Record<SectionKey, SectionGroup>;

export const templatePresetMap = Object.fromEntries(
  sectionGroups.flatMap((group) => group.templates.map((template) => [template.id, template])),
) as Record<string, TemplatePreset>;

export const defaultTemplateIdBySection: Record<SectionKey, string> = {
  proxies: 'proxy-tcp',
  visitors: 'visitor-stcp',
  custom: 'custom-array',
};

export const detectTemplatePresetId = (
  sectionKey: string,
  data: Record<string, unknown>,
): string => {
  if (sectionKey === 'proxies' && typeof data.type === 'string') {
    const templateId = `proxy-${data.type}`;
    if (templatePresetMap[templateId]) {
      return templateId;
    }
  }

  if (sectionKey === 'visitors' && typeof data.type === 'string') {
    const templateId = `visitor-${data.type}`;
    if (templatePresetMap[templateId]) {
      return templateId;
    }
  }

  return defaultTemplateIdBySection.custom;
};
