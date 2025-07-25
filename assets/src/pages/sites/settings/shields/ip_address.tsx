import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { Separator } from "@components/ui/separator"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@components/ui/dialog";
import { Input } from "@components/ui/input";
import { Label } from "@components/ui/label";
import { Textarea } from "@components/ui/textarea";
import axios, { type BaseResponse } from "@utils/axios";
import { AxiosError } from "axios";
import { toast } from "sonner";

// Define interface for IP shield rule
interface ShieldIPRule {
  id: number;
  site_id: number;
  ip: {
    IP: string;
    Mask: string;
  };
  action: string;
  description: string;
  added_by: string;
  created_at: string;
  updated_at: string;
}

// API client for shield management
const fetchShieldIPs = async (domain: string): Promise<ShieldIPRule[]> => {
  try {
    const response = await axios.get<BaseResponse<ShieldIPRule[]>>(`/sites/${domain}/shield/ip`);
    return response.data.data || [];
  } catch (error) {
    console.error('Failed to fetch IP list:', error);
    throw new Error(error instanceof AxiosError ? error.response?.data.error || 'Failed to fetch IP list' : 'Failed to fetch IP list');
  }
};

const addShieldIP = async (domain: string, ip: string, description: string): Promise<ShieldIPRule> => {
  try {
    const response = await axios.post<BaseResponse<ShieldIPRule>>(`/sites/${domain}/shield/ip`, {
      ip,
      description,
      action: 'deny'
    });
    // Check business code in response
    if (response.data.code !== 200) {
      throw new Error(`${response.data.message || 'Operation failed'}: ${response.data.error || ''}`);
    }
    return response.data.data as ShieldIPRule;
  } catch (error) {
    console.error('Failed to add hostname:', error);
    // Extract error message from response
    let errorMsg = 'Failed to add hostname';
    let errorDetail = '';
    // Handle both Axios errors and business custom errors
    if (error instanceof AxiosError) {
      // Handle HTTP errors
      const responseData = error.response?.data || {};
      errorMsg = responseData.message || errorMsg;
      errorDetail = responseData.error || '';
    } else if (error instanceof Error) {
      console.log('error', error)
      errorMsg = error.message || errorMsg;
    }
    if (errorDetail.includes('duplicate key value violates unique constraint')) {
      errorMsg = 'Hostname rule already exists';
      errorDetail = 'A rule with this hostname pattern already exists for this site';
    }
    console.log(errorMsg, errorDetail)
    throw new Error(`${errorMsg}: ${errorDetail}`.trim());
  }
};

interface Params extends Record<string, string | undefined> {
  domain?: string;
}

export default function SettingShieldsIpAddress() {

  const [ipList, setIpList] = useState<ShieldIPRule[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [newIp, setNewIp] = useState('');
  const [newIpDescription, setNewIpDescription] = useState('');
  const [loading, setLoading] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const { domain: domainParam } = useParams<Params>();
  const domain = domainParam!;
  // Handle IP deletion
  const handleDeleteIP = async (id: number) => {
    if (!domain || !window.confirm('Are you sure you want to unblock this IP address?')) return;
    setLoading(true);
    setDeletingId(id);
    try {
      await axios.delete(`/sites/${domain}/shield/ip/${id}`);
      // Refresh IP list
      const updatedIPs = await fetchShieldIPs(domain);
      setIpList(updatedIPs);
      toast.success('IP address unblocked successfully');
    } catch (error) {
      console.error('Error deleting IP address:', error);
      if (error instanceof AxiosError) {
        const status = error.response?.status;
        const errorData = error.response?.data;
        toast.error(`Failed to unblock IP address (${status || 'Unknown'})`, {
          description: errorData?.message || errorData?.error || 'Unknown error'
        });
      } else {
        toast.error('Failed to unblock IP address', {
          description: 'Unknown error'
        });
      }
    } finally {
      setLoading(false);
      setDeletingId(null);
    }
  };

  // Load IP list on component mount and domain change
  useEffect(() => {
    if (!domain) return;
    const loadIPs = async () => {
      setLoading(true);
      try {
        const ips = await fetchShieldIPs(domain);
        setIpList(ips);
      } catch (error) {
        console.error('Error loading IP list:', error);
        toast.error('加载IP地址列表失败', {
          description: error instanceof AxiosError ? error.response?.data.error : '未知错误'
        });
      } finally {
        setLoading(false);
      }
    };
    loadIPs();
  }, [domain]);


  // Handle IP addition
  const handleAddIP = async () => {
    if (!newIp.trim() || !domain) return;
    setLoading(true);
    try {
      await addShieldIP(domain, newIp.trim(), newIpDescription.trim());
      // Refresh IP list
      const updatedIPs = await fetchShieldIPs(domain);
      setIpList(updatedIPs);
      // Show success toast and reset form
      setNewIp('');
      setNewIpDescription('');
      setIsModalOpen(false);
      toast.success('IP address blocked successfully');
    } catch (error) {
      console.error('Error adding IP address:', error);
      const errorMessage = error instanceof Error ? error.message : 'Failed to add IP address';
      toast.error('添加IP地址失败', {
        description: errorMessage
      });
    } finally {
      setLoading(false);
    }
  };

  // Validate domain exists
  if (!domainParam) {
    return <div>Loading...</div>;
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <div>
            <CardTitle>IP Blacklist</CardTitle>
            <CardDescription>Block incoming traffic from specific IP addresses (blacklist).</CardDescription>
          </div>
          <Button onClick={() => setIsModalOpen(true)}>Add Blocked IP</Button>
        </CardHeader>
        <CardContent>
          <Separator className="my-6" />

          {loading ? (
            <div className="text-center py-8">Loading IP addresses...</div>
          ) : ipList.length === 0 ? (
            <div className="text-center py-8 text-gray-500">No IP addresses in blacklist yet</div>
          ) : (
            <div className="space-y-4">
              {ipList.map((ip) => (
                <div key={ip.id} className="flex items-center justify-between p-4 border rounded-lg">
                  <div>
                    <p className="font-medium">{ip.ip.IP}</p>
                    {ip.description && (
                      <p className="text-sm text-gray-500 mt-1">{ip.description}</p>
                    )}
                    <p className="text-xs text-gray-400">Added by {ip.added_by} on {new Date(ip.created_at).toLocaleString()}</p>
                  </div>
                  <button
                    onClick={() => handleDeleteIP(ip.id)}
                    className="text-red-500 hover:text-red-700"
                    disabled={loading}
                  >
                    {loading && ip.id === deletingId ? 'Deleting...' : 'Delete'}
                  </button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Add IP Modal */}
      <Dialog open={isModalOpen} onOpenChange={setIsModalOpen}>
        <DialogContent className="sm:max-w-[425px]">
          <DialogHeader>
            <DialogTitle>Add IP to Blacklist</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="ip-address">IP Address</Label>
              <Input
                id="ip-address"
                value={newIp}
                onChange={(e) => setNewIp(e.target.value)}
                placeholder="Enter IP address"
                disabled={loading}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ip-description">Description (optional)</Label>
              <Textarea
                id="ip-description"
                value={newIpDescription}
                onChange={(e) => setNewIpDescription(e.target.value)}
                placeholder="Why is this IP blocked?"
                className="resize-none"
                rows={3}
                disabled={loading}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setIsModalOpen(false)}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button onClick={handleAddIP} disabled={loading || !newIp.trim()}>
              {loading ? 'Adding...' : 'Add IP'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
