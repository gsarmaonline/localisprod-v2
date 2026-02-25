import { useEffect, useState } from 'react'
import Modal from './Modal'
import {
  providers,
  DOMetadata,
  AWSMetadata,
  Node,
} from '../api/client'

type Provider = 'digitalocean' | 'aws'

interface Props {
  onClose: () => void
  onSuccess: (node: Node) => void
}

export default function ProvisionNodeModal({ onClose, onSuccess }: Props) {
  const [step, setStep] = useState<'pick' | 'form' | 'provisioning'>('pick')
  const [provider, setProvider] = useState<Provider | null>(null)
  const [doMeta, setDoMeta] = useState<DOMetadata | null>(null)
  const [awsMeta, setAwsMeta] = useState<AWSMetadata | null>(null)
  const [metaError, setMetaError] = useState<string | null>(null)
  const [metaLoading, setMetaLoading] = useState(false)
  const [provisionError, setProvisionError] = useState<string | null>(null)

  // DO form state
  const [doForm, setDoForm] = useState({ name: '', region: '', size: '', image: '' })
  // AWS form state
  const [awsForm, setAwsForm] = useState({ name: '', region: '', instance_type: '', os: '' })

  const selectProvider = async (p: Provider) => {
    setProvider(p)
    setMetaError(null)
    setMetaLoading(true)
    try {
      if (p === 'digitalocean') {
        const meta = await providers.doMetadata()
        setDoMeta(meta)
        setDoForm(f => ({
          ...f,
          region: meta.regions[0]?.slug ?? '',
          size: meta.sizes[0]?.slug ?? '',
          image: meta.images[0]?.slug ?? '',
        }))
      } else {
        const meta = await providers.awsMetadata()
        setAwsMeta(meta)
        setAwsForm(f => ({
          ...f,
          region: meta.regions[0]?.id ?? '',
          instance_type: meta.instance_types[0]?.id ?? '',
          os: meta.os_options[0]?.id ?? '',
        }))
      }
      setStep('form')
    } catch (e: unknown) {
      setMetaError((e as Error).message)
    } finally {
      setMetaLoading(false)
    }
  }

  const handleProvision = async () => {
    setProvisionError(null)
    setStep('provisioning')
    try {
      let node: Node
      if (provider === 'digitalocean') {
        node = await providers.doProvision(doForm)
      } else {
        node = await providers.awsProvision(awsForm)
      }
      onSuccess(node)
    } catch (e: unknown) {
      setProvisionError((e as Error).message)
      setStep('form')
    }
  }

  return (
    <Modal title="Provision Cloud Node" onClose={onClose}>
      {step === 'pick' && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">Choose a cloud provider to provision a new VM:</p>
          {metaLoading && (
            <p className="text-sm text-gray-500">Loading provider metadata...</p>
          )}
          {metaError && (
            <div className="p-3 bg-red-50 text-red-700 rounded-lg text-sm">
              {metaError}
            </div>
          )}
          <div className="grid grid-cols-2 gap-4">
            <button
              onClick={() => selectProvider('digitalocean')}
              disabled={metaLoading}
              className="flex flex-col items-center gap-2 p-6 border-2 border-gray-200 rounded-xl hover:border-blue-500 hover:bg-blue-50 transition-colors disabled:opacity-50"
            >
              <svg className="w-10 h-10 text-blue-500" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12.032 0C5.376 0 0 5.376 0 12s5.376 12 12.032 12C18.672 24 24 18.624 24 12 24 5.376 18.672 0 12.032 0zm-.032 19.2v-3.36c2.112 0 3.888-1.68 3.888-3.84s-1.776-3.84-3.888-3.84c-2.16 0-3.888 1.68-3.888 3.84h-3.36c0-3.984 3.264-7.2 7.248-7.2s7.2 3.216 7.2 7.2-3.216 7.2-7.2 7.2z"/>
              </svg>
              <span className="font-semibold text-gray-800">DigitalOcean</span>
              <span className="text-xs text-gray-500">Droplets</span>
            </button>
            <button
              onClick={() => selectProvider('aws')}
              disabled={metaLoading}
              className="flex flex-col items-center gap-2 p-6 border-2 border-gray-200 rounded-xl hover:border-orange-500 hover:bg-orange-50 transition-colors disabled:opacity-50"
            >
              <svg className="w-10 h-10 text-orange-500" viewBox="0 0 24 24" fill="currentColor">
                <path d="M6.763 10.036c0 .296.032.535.088.71.064.176.144.368.256.576.04.063.056.127.056.183 0 .08-.048.16-.152.24l-.503.335a.383.383 0 0 1-.208.072c-.08 0-.16-.04-.239-.112a2.47 2.47 0 0 1-.287-.375 6.18 6.18 0 0 1-.248-.471c-.622.734-1.405 1.101-2.347 1.101-.67 0-1.205-.191-1.596-.574-.391-.384-.59-.894-.59-1.533 0-.678.239-1.23.726-1.644.487-.415 1.133-.623 1.955-.623.272 0 .551.024.846.064.296.04.6.104.918.176v-.583c0-.607-.127-1.03-.375-1.277-.255-.248-.686-.367-1.3-.367-.28 0-.568.031-.863.103-.295.072-.583.16-.862.272a2.287 2.287 0 0 1-.28.104.488.488 0 0 1-.127.023c-.112 0-.168-.08-.168-.247v-.391c0-.128.016-.224.056-.28a.597.597 0 0 1 .224-.167c.279-.144.614-.264 1.005-.36a4.84 4.84 0 0 1 1.246-.151c.95 0 1.644.216 2.091.647.439.43.662 1.085.662 1.963v2.586zm-3.24 1.214c.263 0 .534-.048.822-.144.287-.096.543-.271.758-.51.128-.152.224-.32.272-.512.047-.191.08-.423.08-.694v-.335a6.66 6.66 0 0 0-.735-.136 6.02 6.02 0 0 0-.75-.048c-.535 0-.926.104-1.19.32-.263.215-.39.518-.39.917 0 .375.095.655.295.846.191.2.47.296.838.296zm6.41.862c-.144 0-.24-.024-.304-.08-.063-.048-.12-.16-.168-.311L7.586 5.55a1.398 1.398 0 0 1-.072-.32c0-.128.064-.2.191-.2h.783c.152 0 .255.025.31.08.065.048.113.16.16.312l1.342 5.284 1.245-5.284c.04-.16.088-.264.151-.312a.549.549 0 0 1 .32-.08h.638c.152 0 .256.025.32.08.063.048.12.16.151.312l1.261 5.348 1.381-5.348c.048-.16.104-.264.16-.312a.52.52 0 0 1 .311-.08h.743c.127 0 .2.065.2.2 0 .04-.009.08-.017.128a1.137 1.137 0 0 1-.056.2l-1.923 6.17c-.048.16-.104.263-.168.311a.51.51 0 0 1-.303.08h-.687c-.151 0-.255-.024-.32-.08-.063-.056-.119-.16-.15-.32l-1.238-5.148-1.23 5.14c-.04.16-.087.264-.15.32-.065.056-.177.08-.32.08zm10.256.215c-.415 0-.83-.048-1.229-.143-.399-.096-.71-.2-.918-.32-.128-.071-.215-.151-.247-.223a.563.563 0 0 1-.048-.224v-.407c0-.167.064-.247.183-.247.048 0 .096.008.144.024.048.016.12.048.2.08.271.12.566.215.878.279.319.064.63.096.95.096.502 0 .894-.088 1.165-.264a.86.86 0 0 0 .415-.758.777.777 0 0 0-.215-.559c-.144-.151-.416-.287-.807-.415l-1.157-.36c-.583-.183-1.014-.454-1.277-.813a1.902 1.902 0 0 1-.4-1.158c0-.335.073-.63.216-.886.144-.255.335-.479.575-.654.24-.184.51-.32.83-.415.32-.096.655-.136 1.006-.136.175 0 .359.008.535.032.183.024.35.056.518.088.16.04.312.08.455.127.144.048.256.096.336.144a.69.69 0 0 1 .24.2.43.43 0 0 1 .071.263v.375c0 .168-.064.256-.184.256a.83.83 0 0 1-.303-.096 3.652 3.652 0 0 0-1.532-.311c-.455 0-.815.072-1.062.224-.248.152-.375.383-.375.71 0 .224.08.416.24.567.159.152.454.304.877.44l1.134.358c.574.184.99.44 1.237.767.247.327.367.702.367 1.117 0 .343-.072.655-.207.926-.144.272-.336.511-.583.703-.248.2-.543.343-.886.447-.36.111-.743.167-1.142.167z"/>
              </svg>
              <span className="font-semibold text-gray-800">AWS</span>
              <span className="text-xs text-gray-500">EC2 Instances</span>
            </button>
          </div>
        </div>
      )}

      {step === 'form' && provider === 'digitalocean' && doMeta && (
        <div className="space-y-4">
          <button
            onClick={() => { setStep('pick'); setProvider(null) }}
            className="text-sm text-blue-600 hover:text-blue-800 flex items-center gap-1"
          >
            ← Back
          </button>

          {provisionError && (
            <div className="p-3 bg-red-50 text-red-700 rounded-lg text-sm">{provisionError}</div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Node Name</label>
            <input
              type="text"
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={doForm.name}
              onChange={e => setDoForm(f => ({ ...f, name: e.target.value }))}
              placeholder="my-droplet"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Region</label>
            <select
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={doForm.region}
              onChange={e => setDoForm(f => ({ ...f, region: e.target.value }))}
            >
              {doMeta.regions.map(r => (
                <option key={r.slug} value={r.slug}>{r.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Size</label>
            <select
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={doForm.size}
              onChange={e => setDoForm(f => ({ ...f, size: e.target.value }))}
            >
              {doMeta.sizes.map(s => (
                <option key={s.slug} value={s.slug}>{s.description} (${s.price_monthly}/mo)</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">OS</label>
            <select
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={doForm.image}
              onChange={e => setDoForm(f => ({ ...f, image: e.target.value }))}
            >
              {doMeta.images.map(img => (
                <option key={img.slug} value={img.slug}>{img.name}</option>
              ))}
            </select>
          </div>

          <div className="flex gap-2 justify-end pt-2">
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900"
            >
              Cancel
            </button>
            <button
              onClick={handleProvision}
              disabled={!doForm.name || !doForm.region || !doForm.size || !doForm.image}
              className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50"
            >
              Provision Droplet
            </button>
          </div>
        </div>
      )}

      {step === 'form' && provider === 'aws' && awsMeta && (
        <div className="space-y-4">
          <button
            onClick={() => { setStep('pick'); setProvider(null) }}
            className="text-sm text-orange-600 hover:text-orange-800 flex items-center gap-1"
          >
            ← Back
          </button>

          {provisionError && (
            <div className="p-3 bg-red-50 text-red-700 rounded-lg text-sm">{provisionError}</div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Node Name</label>
            <input
              type="text"
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
              value={awsForm.name}
              onChange={e => setAwsForm(f => ({ ...f, name: e.target.value }))}
              placeholder="my-ec2-instance"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Region</label>
            <select
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
              value={awsForm.region}
              onChange={e => setAwsForm(f => ({ ...f, region: e.target.value }))}
            >
              {awsMeta.regions.map(r => (
                <option key={r.id} value={r.id}>{r.name} ({r.id})</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Instance Type</label>
            <select
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
              value={awsForm.instance_type}
              onChange={e => setAwsForm(f => ({ ...f, instance_type: e.target.value }))}
            >
              {awsMeta.instance_types.map(t => (
                <option key={t.id} value={t.id}>{t.description}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">OS</label>
            <select
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
              value={awsForm.os}
              onChange={e => setAwsForm(f => ({ ...f, os: e.target.value }))}
            >
              {awsMeta.os_options.map(o => (
                <option key={o.id} value={o.id}>{o.name}</option>
              ))}
            </select>
          </div>

          <div className="flex gap-2 justify-end pt-2">
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900"
            >
              Cancel
            </button>
            <button
              onClick={handleProvision}
              disabled={!awsForm.name || !awsForm.region || !awsForm.instance_type || !awsForm.os}
              className="px-4 py-2 bg-orange-600 text-white text-sm rounded-lg hover:bg-orange-700 disabled:opacity-50"
            >
              Provision Instance
            </button>
          </div>
        </div>
      )}

      {step === 'provisioning' && (
        <div className="flex flex-col items-center gap-4 py-8">
          <div className="w-10 h-10 border-4 border-blue-500 border-t-transparent rounded-full animate-spin" />
          <p className="text-sm font-medium text-gray-800">Provisioning your node...</p>
          <p className="text-xs text-gray-500 text-center">
            Creating the instance and waiting for it to start.<br />
            This may take 1–2 minutes.
          </p>
        </div>
      )}
    </Modal>
  )
}
