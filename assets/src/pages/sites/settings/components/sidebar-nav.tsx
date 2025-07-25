import { cn } from "@/lib/utils"
import { buttonVariants } from "@components/ui/variants/button-variants"
import { useLocation } from "react-router-dom"
import { Link } from "react-router-dom";


export interface SidebarNavItem {
  id: string;
  icon?: React.ReactNode;
  href?: string;
  title: string;
  children?: SidebarNavItem[];
}

interface SidebarNavProps extends React.HTMLAttributes<HTMLElement> {
  items: SidebarNavItem[];
}

export function SidebarNav({ className, items, ...props }: SidebarNavProps) {
  const location = useLocation();

  const renderNavItem = (item: SidebarNavItem) => {
    const isActive = location.pathname === item.href ||
      (item.children?.some(child => child.href && location.pathname.startsWith(child.href)) ?? false);
    return (
      <div key={item.id} className="relative">
        {item.href ? (
          <Link
            to={item.href}
            className={cn(
              buttonVariants({ variant: "ghost" }),
              isActive
                ? "bg-muted hover:bg-muted"
                : "hover:bg-transparent hover:underline",
              "justify-start"
            )}
          >
            {item.icon}
            {item.title}
          </Link>
        ) : (
          <div
            className={cn(
              buttonVariants({ variant: "ghost" }),
              "justify-start cursor-default",
              isActive ? "bg-muted" : ""
            )}
          >
            {item.icon}
            {item.title}
          </div>
        )}

        {item.children && (
          <div className="pl-4 mt-1 space-y-1">
            {item.children.map(child => renderNavItem(child))}
          </div>
        )}
      </div>
    );
  };

  return (
    <nav
      className={cn(
        "flex space-x-2 lg:flex-col lg:space-x-0 lg:space-y-1",
        className
      )}
      {...props}
    >
      {items.map(item => renderNavItem(item))}
    </nav>
  )
}
