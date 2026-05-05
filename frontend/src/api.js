const BASE = '/api'

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' }
  const token = localStorage.getItem('token')
  if (token) headers['Authorization'] = `Bearer ${token}`

  const opts = { method, headers }
  if (body) opts.body = JSON.stringify(body)

  const res = await fetch(`${BASE}${path}`, opts)
  if (res.status === 401) {
    localStorage.removeItem('token')
    window.location.href = '/login'
    throw new Error('unauthorized')
  }
  const ct = res.headers.get('Content-Type') || ''
  if (ct.includes('application/json')) return res.json()
  return res.text()
}

export const api = {
  request,
  login: (data) => request('POST', '/auth/login', data),
  health: () => request('GET', '/health'),

  // sources
  listSources: () => request('GET', '/sources'),
  createSource: (data) => request('POST', '/sources', data),
  getSource: (id) => request('GET', `/sources/${id}`),
  updateSource: (id, data) => request('PUT', `/sources/${id}`, data),
  deleteSource: (id) => request('DELETE', `/sources/${id}`),
  testSource: (url) => request('GET', `/sources/test?url=${encodeURIComponent(url)}`),
  testAndSaveSource: (id) => request('POST', `/sources/${id}/test`),

  // nodes (V3)
  parseNodeLink: (links) => request('POST', '/nodes/parse', { links }),
  listNodes: () => request('GET', '/nodes'),
  createNode: (data) => request('POST', '/nodes', data),
  getNode: (id) => request('GET', `/nodes/${id}`),
  updateNode: (id, data) => request('PUT', `/nodes/${id}`, data),
  deleteNode: (id) => request('DELETE', `/nodes/${id}`),
  testNode: (id) => request('POST', `/nodes/${id}/test`),
  reorderNodes: (orderedIds) => request('POST', '/nodes/reorder', { ordered_ids: orderedIds }),
  latencyBest: (domain) => request('GET', `/latency/best?domain=${encodeURIComponent(domain)}`),

  // domain mappings (local opt)
  listDomainMappings: () => request('GET', '/domain-mappings'),
  createDomainMapping: (data) => request('POST', '/domain-mappings', data),
  batchCreateDomainMappings: (data) => request('POST', '/domain-mappings/batch', data),
  updateDomainMapping: (id, data) => request('PUT', `/domain-mappings/${id}`, data),
  deleteDomainMapping: (id) => request('DELETE', `/domain-mappings/${id}`),
  pingDomain: (domain) => request('GET', `/domains/ping?domain=${encodeURIComponent(domain)}`),

  // CF bindings (per-node multi 优选)
  listCFBindings: (nodeID) => request('GET', `/nodes/${nodeID}/cf-bindings`),
  createCFBinding: (nodeID, data) => request('POST', `/nodes/${nodeID}/cf-bindings`, data),
  deleteCFBinding: (nodeID, bid) => request('DELETE', `/nodes/${nodeID}/cf-bindings/${bid}`),

  // groups (legacy)
  listGroups: () => request('GET', '/groups'),
  createGroup: (data) => request('POST', '/groups', data),
  getGroup: (id) => request('GET', `/groups/${id}`),
  updateGroup: (id, data) => request('PUT', `/groups/${id}`, data),
  generateConfig: (id, type = 'clash') => request('GET', `/generate/${id}?type=${type}`),

  latencyData: (hours) => request('GET', '/latency?hours=' + (hours || 24)),
  latencyDailyPicks: () => request('GET', '/latency/daily-picks'),
  latencyByIP: (ip, hours) => request('GET', '/latency/ip?ip=' + encodeURIComponent(ip) + '&hours=' + (hours || 24)),
  latencyBestDetail: (domain) => request('GET', '/latency/best-detail?domain=' + encodeURIComponent(domain)),

  // backups
  listBackups: () => request('GET', '/backups'),
  createBackup: (data) => request('POST', '/backups', data),
  restoreBackup: (id) => request('POST', `/backups/${id}/restore`),

  // subscription links
  listSubLinks: () => request('GET', '/sub-links'),
  createSubLink: (data) => request('POST', '/sub-links', data),
  updateSubLink: (hash, data) => request('PUT', `/sub-links/${hash}`, data),
  deleteSubLink: (hash) => request('DELETE', `/sub-links/${hash}`),

  // speed clients
  listSpeedClients: () => request('GET', '/speed-clients'),
  upsertSpeedClient: (data) => request('PUT', '/speed-clients', data),
  deleteClientRecords: (clientId) => request('DELETE', `/speed-clients/${clientId}/records`),

  // algorithms
  listAlgorithms: () => request('GET', '/algorithms'),
  createAlgorithm: (data) => request('POST', '/algorithms', data),
  updateAlgorithm: (id, data) => request('PUT', `/algorithms/${id}`, data),
  deleteAlgorithm: (id) => request('DELETE', `/algorithms/${id}`),

  // traffic
  trafficOverview: () => request('GET', '/traffic/overview').catch(() => ({ used_bytes: 0, quota_bytes: 0 })),

  // admin
  adminListUsers: () => request('GET', '/admin/users'),
  adminUpdateUser: (id, data) => request('PUT', `/admin/users/${id}`, data),
  adminDeleteUser: (id) => request('DELETE', `/admin/users/${id}`),
  adminResetTraffic: (id) => request('POST', `/admin/users/${id}/reset-traffic`),
}
