import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardAction,
} from "@/components/ui/card";
import { Settings } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useEffect, useState, useCallback } from "react";
import axios, { type BaseResponse } from "@utils/axios";

// 自定义防抖 Hook
const useDebounce = (value: string, delay: number) => {
  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => {
      clearTimeout(handler);
    };
  }, [value, delay]);

  return debouncedValue;
};

// 将 Site 接口定义提前
interface Site {
  id: number;
  domain: string;
  remark: string;
  role: string;
}

export default function Sites() {
  const navigate = useNavigate();
  const [sites, setSites] = useState([] as Site[]);
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedSearchQuery = useDebounce(searchQuery, 300);

  const fetchSites = useCallback(async () => {
    const url = debouncedSearchQuery
      ? `/sites?domain=${encodeURIComponent(debouncedSearchQuery)}`
      : "/sites";
    try {
      const res = await axios.get<BaseResponse<Site[]>>(url);
      console.log(res);
      if (res.status === 200) {
        setSites(res.data.data);
      }
    } catch (error) {
      console.error("Failed to fetch sites:", error);
    }
  }, [debouncedSearchQuery]);

  useEffect(() => {
    fetchSites();
  }, [fetchSites]);

  return (
    <div className="container mx-auto px-4 py-8">
      {/* 标题部分 */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">站点管理</h1>
        <div className="border-b border-gray-200 mt-4"></div>
      </div>

      {/* 搜索和添加按钮行 */}
      <div className="flex justify-between mb-6">
        <div className="w-1/4">
          <Input
            placeholder="搜索站点..."
            value={searchQuery}
            className="bg-white"
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
        <Button variant="default" onClick={() => navigate("/sites/new")}>
          添加站点
        </Button>
      </div>

      {/* 站点卡片列表 */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {sites.map((site) => (
          <Card key={site.id}>
            <CardHeader className="relative">
              <CardTitle>{site.domain}</CardTitle>
              <CardDescription>{site.remark}</CardDescription>
              <CardAction>
                <Button
                  disabled={site.role === "viewer"}
                  variant="link"
                  className="cursor-pointer hover:cursor-pointer text-gray-400 dark:text-gray-600 hover:text-black dark:hover:text-indigo-40"
                >
                  <Settings />
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent>{/* 这里暂时先显示一个空卡片 */}</CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
