import socket
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(3)
s.sendto(b'{"method":"getPilot"}', ("192.168.1.13", 38899))
print(s.recv(4096).decode())