import { Separator } from "@components/ui/separator"
import { SidebarNav } from "./components/sidebar-nav"
import { useNavigate, useParams } from "react-router-dom";
import { Rocket, Users } from "lucide-react";

interface SettingsLayoutProps {
  children: React.ReactNode
}

export default function SettingsLayout({ children }: SettingsLayoutProps) {

  const { domain } = useParams();
  const navigate = useNavigate();
  const sidebarNavItems = [
    {
      title: "General",
      icon: <Rocket />,
      href: `/sites/pcsoft.com.cn/settings/general`,
    },
    {
      title: "People",
      icon: <Users />,
      href: `/sites/pcsoft.com.cn/settings/people`,
    },
  ]

  return (
    <>
      <div className="hidden space-y-6 p-10 pb-16 md:block">
        <div className="space-y-0.5">
          <div
            className="text-blue-500 hover:underline cursor-pointer w-fit"
            onClick={() => navigate(`/sites/${domain}/stats`)}
          >
            ← 返回仪表盘
          </div>
          <h2 className="text-3xl font-bold tracking-tight">Setting for {domain}</h2>
        </div>
        <Separator className="my-6" />
        <div className="flex flex-col space-y-8 lg:flex-row lg:space-x-12 lg:space-y-0">
          <aside className="-mx-4 lg:w-1/5">
            <SidebarNav items={sidebarNavItems} />
          </aside>
          {/* <div className="flex-1 lg:max-w-2xl">{children}</div> */}
           <div className="flex-1 p-4">
        {children}
      </div>
        </div>
      </div>
    </>
  )
}
