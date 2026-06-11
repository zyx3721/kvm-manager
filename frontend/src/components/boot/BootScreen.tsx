import { useBaseConfig } from '../../lib/branding';

export default function BootScreen() {
  const baseConfig = useBaseConfig();
  return (
    <section
      className="fixed inset-0 z-50 flex items-center justify-center overflow-hidden"
      style={{
        background:
          'radial-gradient(circle at 50% 18%, rgba(59,130,246,0.24), transparent 28%), radial-gradient(circle at 25% 70%, rgba(6,182,212,0.16), transparent 30%), var(--kvm-login-bg)',
        color: 'var(--kvm-text)',
      }}
      aria-label="页面加载中"
    >
      <div className="kvm-login-grid absolute inset-0" aria-hidden="true" />
      <div
        className="kvm-login-orb absolute left-1/2 top-1/2 h-72 w-72 -translate-x-1/2 -translate-y-1/2 rounded-full"
        aria-hidden="true"
      />
      <div className="relative flex flex-col items-center gap-5">
        <div className="kvm-boot-ring flex flex-col items-center justify-center gap-2">
          <img
            className="kvm-boot-icon h-20 w-20"
            src={baseConfig.iconData}
            alt={baseConfig.siteName}
          />
          <span className="kvm-boot-shadow" aria-hidden="true" />
        </div>
        <div className="text-center">
          <div className="kvm-gradient-text text-lg font-bold">{baseConfig.siteName}</div>
          <div
            className="mt-1 text-xs uppercase tracking-[0.28em]"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            正在启动控制台
          </div>
        </div>
      </div>
    </section>
  );
}
