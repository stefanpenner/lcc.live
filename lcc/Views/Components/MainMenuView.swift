import SwiftUI

/// Main menu bar with connection status, tab switcher, and grid mode toggle
struct MainMenuView<PopoverContent: View>: View {
    @Binding var selectedTab: MainView.Tab
    @Binding var gridMode: PhotoTabView.GridMode
    @Binding var showConnectionDetails: Bool
    
    let statusColor: Color
    let popoverContent: () -> PopoverContent
    
    init(
        selectedTab: Binding<MainView.Tab>,
        gridMode: Binding<PhotoTabView.GridMode>,
        showConnectionDetails: Binding<Bool>,
        statusColor: Color,
        @ViewBuilder popoverContent: @escaping () -> PopoverContent
    ) {
        self._selectedTab = selectedTab
        self._gridMode = gridMode
        self._showConnectionDetails = showConnectionDetails
        self.statusColor = statusColor
        self.popoverContent = popoverContent
    }
    
    var body: some View {
        HStack(spacing: 0) {
            // Status button
            Button {
                showConnectionDetails.toggle()
            } label: {
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)
                    .shadow(color: statusColor.opacity(0.6), radius: 4)
                    .frame(maxWidth: .infinity)
                    .frame(height: 44)
            }
            .popover(isPresented: $showConnectionDetails) {
                popoverContent()
            }
            
            // LCC Tab button
            Button {
                withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                    selectedTab = .lcc
                }
            } label: {
                Text("LCC")
                    .font(.system(size: 15, weight: selectedTab == .lcc ? .semibold : .regular))
                    .foregroundStyle(selectedTab == .lcc ? .primary : .secondary)
                    .frame(maxWidth: .infinity)
                    .frame(height: 44)
            }
            
            // BCC Tab button
            Button {
                withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                    selectedTab = .bcc
                }
            } label: {
                Text("BCC")
                    .font(.system(size: 15, weight: selectedTab == .bcc ? .semibold : .regular))
                    .foregroundStyle(selectedTab == .bcc ? .primary : .secondary)
                    .frame(maxWidth: .infinity)
                    .frame(height: 44)
            }
            
            // Grid mode toggle button
            Button {
                withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                    gridMode = gridMode == .single ? .compact : .single
                }
            } label: {
                Image(systemName: gridMode == .single ? "square.grid.2x2" : "rectangle.fill")
                    .font(.system(size: 16, weight: .medium))
                    .foregroundStyle(.primary)
                    .frame(maxWidth: .infinity)
                    .frame(height: 44)
            }
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 8)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 20))
        .padding(.horizontal, 20)
        .padding(.bottom, 40)
    }
}

#Preview {
    ZStack {
        Color.black.ignoresSafeArea()
        
        VStack {
            Spacer()
            
            MainMenuView(
                selectedTab: .constant(.lcc),
                gridMode: .constant(.single),
                showConnectionDetails: .constant(false),
                statusColor: .green
            ) {
                Text("Connection Details")
                    .padding()
            }
        }
    }
}

#Preview("Different States") {
    ZStack {
        Color.black.ignoresSafeArea()
        
        VStack(spacing: 40) {
            // LCC selected, connected state
            MainMenuView(
                selectedTab: .constant(.lcc),
                gridMode: .constant(.single),
                showConnectionDetails: .constant(false),
                statusColor: .green
            ) {
                Text("Connected")
                    .padding()
            }
            
            // BCC selected, error state
            MainMenuView(
                selectedTab: .constant(.bcc),
                gridMode: .constant(.compact),
                showConnectionDetails: .constant(false),
                statusColor: .red
            ) {
                Text("Error")
                    .padding()
            }
            
            // Loading state, compact grid
            MainMenuView(
                selectedTab: .constant(.lcc),
                gridMode: .constant(.compact),
                showConnectionDetails: .constant(false),
                statusColor: .orange
            ) {
                Text("Loading")
                    .padding()
            }
        }
    }
}

