import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarImage } from "@/components/ui/avatar";
import Logo from "@/assets/logo.svg";
import md5 from 'md5';

interface User {
  email: string;
  name: string;
}

// 模拟用户状态
const useUser = () => {
  const navigate = useNavigate();
  const [user, setUser] = useState<User | null>(null);

  const set = (user: User) => {
    setUser(user);
  };

  const logout = () => {
    setUser(null);
    localStorage.removeItem("email")
    localStorage.removeItem("name")
    localStorage.removeItem("token")
    localStorage.removeItem("refreshToken")
    navigate("/login")
  };

  return { user, set, logout };
};

// 模拟下拉菜单组件
const UserDropdown = ({
  user,
  onLogout,
}: {
  user: { name: string };
  onLogout: () => void;
}) => {
  const navigate = useNavigate();
  const avatar = "https://www.gravatar.com/avatar/" + md5(user.name) + "?s=150&d=identicon"
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <div
          className="inline-flex items-center w-full justify-center rounded-md bg-white px-4 py-2 text-sm font-medium text-gray-700 cursor-pointer"
        >
          {user.name || "未登录"}
          <Avatar className="h-6 w-6 ml-1" hidden={!user.name}>
            <AvatarImage src={avatar} />
          </Avatar>
        </div>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-56">
        <div className="px-2 py-1.5">
          <div className="text-sm text-muted-foreground">Signed in as</div>
          <div className="text-sm font-medium">wrpota@gmail.com</div>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => navigate("/settings")}>
          设置
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onLogout}>退出登录</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};

export default function Header() {
  const { user, logout, set } = useUser();
  const location = useLocation();

  const navigate = useNavigate();

  useEffect(() => {
    const user = {
      "email": localStorage.getItem("email") || "",
      "name": localStorage.getItem("name") || "",
    }
    set(user)
  }, [location.pathname]);
  return (
    <header className="bg-white shadow">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between h-16">
          <div className="flex">
            <div className="flex-shrink-0 flex items-center">
              <img
                className="block h-8 w-auto"
                src={Logo}
                alt="Your Company"
              />
            </div>
            <div className="flex items-center ml-4">
              <span className="text-gray-700 font-medium">ZenStats</span>
            </div>
          </div>
          <div className="flex items-center">
            {user ? (
              <UserDropdown user={user} onLogout={logout} />
            ) : (
              <button
                type="button"
                className="inline-flex items-center w-full justify-center rounded-md bg-white px-4 py-2 text-sm font-medium text-gray-700 cursor-pointer"
                onClick={() => navigate("/login")}
              >
                登录
              </button>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
