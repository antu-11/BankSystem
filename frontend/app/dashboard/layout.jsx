import Sidebar from '@/components/Sidebar';

export const metadata = {
  title: 'Dashboard — The Vault',
  description: 'Manage your accounts, transfers, and transaction history.',
};

export default function DashboardLayout({ children }) {
  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        <div className="max-w-6xl mx-auto p-6 lg:p-8">
          {children}
        </div>
      </main>
    </div>
  );
}
