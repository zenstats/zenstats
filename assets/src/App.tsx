import { useRoutes } from "react-router-dom";
import { Toaster } from "@/components/ui/sonner"
import Header from "@/components/Header";
import routes from "./routes";
import "./App.css";

export default function App() {
  return (
    <div>
      <Header />
      <Toaster
        position="top-right"
        richColors
        toastOptions={{
        }}
      />
      <div className="flex min-h-[calc(100vh-64px)] bg-gray-100">
        <div className="max-w-7xl w-full mx-auto">
          {useRoutes(routes)}
        </div>
      </div>
    </div>
  );
}