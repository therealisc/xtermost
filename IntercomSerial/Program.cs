using System;
using System.IO;
using System.IO.Ports;

class SerialLogger
{
    static void Main()
    {
        const string portPath = "/dev/ttyACM0";   // Must be full path on Linux
        const int baudRate = 115200;              // Adjust if needed
        const string logFile = "serial_log.txt";

        SerialPort port = new SerialPort(portPath, baudRate);

        // Optional configuration
        port.Parity = Parity.None;
        port.StopBits = StopBits.One;
        port.DataBits = 8;
        port.Handshake = Handshake.None;
        port.NewLine = "\n";

        try
        {
            port.Open();
            Console.WriteLine("Listening on " + portPath);

            using (StreamWriter log = new StreamWriter(logFile, append: true))
            {
                while (true)
                {
                    try
                    {
                        string line = port.ReadLine();  // waits for newline
                        string timestamp = DateTime.UtcNow.ToString("o");
                        string formatted = $"{timestamp} | {line}";

                        log.WriteLine(formatted);
                        log.Flush();

                        Console.WriteLine(formatted);
                    }
                    catch (TimeoutException)
                    {
                        // Not fatal; continue reading
                    }
                }
            }
        }
        catch (UnauthorizedAccessException ex)
        {
            Console.WriteLine("Access denied to port. Try: sudo usermod -a -G dialout <your-user>");
            Console.WriteLine(ex.Message);
        }
        catch (Exception ex)
        {
            Console.WriteLine("Error opening or reading the port:");
            Console.WriteLine(ex);
        }
        finally
        {
            if (port.IsOpen)
                port.Close();
        }
    }
}
