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
  group?: 'basic' | 'advanced';
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
  booleanField('transport.useEncryption', '启用加密', {
    help: '对传输内容做加密，适合公网链路。',
  }),
  booleanField('transport.useCompression', '启用压缩', {
    help: '带宽紧张时可开启，CPU 会略有增加。',
  }),
  textField('transport.bandwidthLimit', '带宽限制', {
    placeholder: '1MB',
    help: '例如 1MB、500KB。',
  }),
  selectField(
    'transport.bandwidthLimitMode',
    '限速位置',
    [
      { label: 'client', value: 'client' },
      { label: 'server', value: 'server' },
    ],
  ),
  selectField(
    'transport.proxyProtocolVersion',
    'Proxy Protocol 版本',
    [
      { label: 'v1', value: 'v1' },
      { label: 'v2', value: 'v2' },
    ],
  ),
  numberField('transport.poolCount', '连接池数量', {
    placeholder: '5',
  }),
];

const proxyLoadBalancerFields: FieldSchema[] = [
  textField('loadBalancer.group', '负载均衡组', {
    placeholder: 'web-group',
  }),
  textField('loadBalancer.groupKey', '负载均衡密钥', {
    placeholder: 'group-secret',
  }),
];

const proxyHealthCheckFields: FieldSchema[] = [
  selectField('healthCheck.type', '健康检查类型', healthCheckOptions),
  numberField('healthCheck.timeoutSeconds', '检查超时秒数', {
    placeholder: '3',
  }),
  numberField('healthCheck.maxFailed', '最大失败次数', {
    placeholder: '1',
  }),
  numberField('healthCheck.intervalSeconds', '检查间隔秒数', {
    placeholder: '10',
  }),
  textField('healthCheck.path', '健康检查路径', {
    placeholder: '/healthz',
    showWhen: isHealthCheckType('http'),
  }),
];

const proxyPluginFields: FieldSchema[] = [
  selectField('plugin.type', '插件类型', pluginOptions),
  textField('plugin.httpUser', 'HTTP 用户名', {
    placeholder: 'demo',
    showWhen: isPluginType('http_proxy', 'static_file'),
  }),
  textField('plugin.httpPassword', 'HTTP 密码', {
    placeholder: 'password',
    showWhen: isPluginType('http_proxy', 'static_file'),
  }),
  textField('plugin.username', '插件用户名', {
    placeholder: 'demo',
    showWhen: isPluginType('socks5'),
  }),
  textField('plugin.password', '插件密码', {
    placeholder: 'password',
    showWhen: isPluginType('socks5'),
  }),
  textField('plugin.localPath', '本地目录', {
    placeholder: '/var/www/html',
    showWhen: isPluginType('static_file'),
  }),
  textField('plugin.stripPrefix', '剥离路径前缀', {
    placeholder: '/static',
    showWhen: isPluginType('static_file'),
  }),
  textField('plugin.unixPath', 'Unix Socket 路径', {
    placeholder: '/var/run/docker.sock',
    showWhen: isPluginType('unix_domain_socket'),
  }),
  textField('plugin.localAddr', '插件本地地址', {
    placeholder: '127.0.0.1:8080',
    showWhen: isPluginType('http2https', 'https2http', 'https2https', 'tls2raw'),
  }),
  textField('plugin.hostHeaderRewrite', '重写 Host', {
    placeholder: 'example.com',
    showWhen: isPluginType('http2https', 'https2http', 'https2https'),
  }),
  textField('plugin.crtPath', '证书路径', {
    placeholder: '/path/to/fullchain.pem',
    showWhen: isPluginType('http2https', 'https2http', 'https2https', 'tls2raw'),
  }),
  textField('plugin.keyPath', '私钥路径', {
    placeholder: '/path/to/privkey.pem',
    showWhen: isPluginType('http2https', 'https2http', 'https2https', 'tls2raw'),
  }),
  booleanField('plugin.enableHTTP2', '启用 HTTP/2', {
    showWhen: isPluginType('https2https'),
  }),
  textField('plugin.network', '虚拟网络网段', {
    placeholder: '192.168.111.0/24',
    showWhen: isPluginType('virtual_net'),
  }),
];

const proxyOptionalFields: FieldSchema[] = [
  ...proxyTransportFields,
  ...proxyLoadBalancerFields,
  ...proxyHealthCheckFields,
  ...proxyPluginFields,
].map((field) => ({ ...field, group: 'advanced' as const }));

const visitorBaseFields: FieldSchema[] = [
  textField('name', '名称', {
    required: true,
    placeholder: 'visit_a_ssh',
  }),
  textField('serverName', '服务端代理名称', {
    required: true,
    placeholder: 'device_a_ssh',
  }),
  textField('secretKey', '访问密钥', {
    required: true,
    placeholder: 'shared-secret',
  }),
  textField('bindAddr', '本地监听地址', {
    defaultValue: '127.0.0.1',
    placeholder: '127.0.0.1',
  }),
  numberField('bindPort', '本地监听端口', {
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
      numberField('remotePort', '远程端口', {
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
      numberField('remotePort', '远程端口', {
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
      arrayField('customDomains', '绑定域名', {
        placeholder: 'a.example.com, b.example.com',
      }),
      textField('subdomain', '子域名', {
        placeholder: 'demo-web',
      }),
      arrayField('locations', '路径匹配', {
        placeholder: '/,/admin',
      }),
      textField('hostHeaderRewrite', '重写 Host', {
        placeholder: 'internal.service.local',
      }),
      textField('httpUser', 'HTTP 用户名', {
        placeholder: 'demo',
      }),
      textField('httpPassword', 'HTTP 密码', {
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
      arrayField('customDomains', '绑定域名', {
        placeholder: 'secure.example.com',
      }),
      textField('subdomain', '子域名', {
        placeholder: 'secure-app',
      }),
      textField('hostHeaderRewrite', '重写 Host', {
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
      selectField('multiplexer', '复用协议', tcpMuxMultiplexerOptions, {
        defaultValue: 'httpconnect',
      }),
      arrayField('customDomains', '绑定域名', {
        placeholder: 'mux.example.com',
      }),
      textField('subdomain', '子域名', {
        placeholder: 'mux-service',
      }),
      textField('httpUser', 'HTTP 用户名', {
        placeholder: 'demo',
      }),
      textField('httpPassword', 'HTTP 密码', {
        placeholder: 'password',
      }),
      textField('routeByHTTPUser', '按 HTTP 用户路由', {
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
      textField('secretKey', '访问密钥', {
        required: true,
        placeholder: 'shared-secret',
      }),
      arrayField('allowUsers', '允许的用户', {
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
      textField('secretKey', '访问密钥', {
        required: true,
        placeholder: 'shared-secret',
      }),
      arrayField('allowUsers', '允许的用户', {
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
      textField('secretKey', '访问密钥', {
        required: true,
        placeholder: 'shared-secret',
      }),
      arrayField('allowUsers', '允许的用户', {
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
      selectField('protocol', '打洞协议', xtcpProtocolOptions),
      booleanField('keepTunnelOpen', '保持隧道打开'),
      textField('fallbackTo', '失败回退地址', {
        placeholder: '127.0.0.1:22',
      }),
      numberField('fallbackTimeoutMs', '回退超时毫秒', {
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
