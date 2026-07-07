import { Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { useAuth } from "./context/AuthContext";
import { DomainDetailPage } from "./pages/DomainDetailPage";
import { DomainFormPage } from "./pages/DomainFormPage";
import { DomainsPage } from "./pages/DomainsPage";
import { LoginPage } from "./pages/LoginPage";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route path="/" element={<DomainsPage />} />
        <Route path="/domains/new" element={<DomainFormPage />} />
        <Route path="/domains/:id" element={<DomainDetailPage />} />
        <Route path="/domains/:id/edit" element={<DomainFormPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
